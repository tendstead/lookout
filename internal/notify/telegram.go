// SPDX-License-Identifier: Elastic-2.0
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	Token  string
	ChatID string
}

func Load(file string) (*Config, error) {
	if file == "" {
		return nil, nil
	}
	data, e := os.ReadFile(file)
	if e != nil {
		return nil, errors.New("Telegram configuration unavailable")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	if len(data) > 16384 {
		return nil, errors.New("Telegram configuration exceeds limit")
	}
	var raw struct {
		Token string          `json:"bot_token"`
		Chat  json.RawMessage `json:"chat_id"`
	}
	if e = json.Unmarshal(data, &raw); e != nil {
		return nil, errors.New("invalid Telegram configuration")
	}
	var chat string
	if e = json.Unmarshal(raw.Chat, &chat); e != nil {
		var n json.Number
		if e = json.Unmarshal(raw.Chat, &n); e != nil {
			return nil, errors.New("invalid Telegram chat identifier")
		}
		chat = n.String()
	}
	return NewConfig(raw.Token, chat)
}

func NewConfig(token, chat string) (*Config, error) {
	token = strings.TrimSpace(token)
	chat = strings.TrimSpace(chat)
	if !regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`).MatchString(token) || len(token) > 1024 || !regexp.MustCompile(`^(-?[0-9]+|@[A-Za-z0-9_]+)$`).MatchString(chat) {
		return nil, errors.New("invalid Telegram credentials")
	}
	return &Config{Token: token, ChatID: chat}, nil
}

type Client struct {
	BaseURL string
	Token   string
	ChatID  string
	HTTP    *http.Client
}
type APIError struct {
	Code       int
	RetryAfter time.Duration
}

func (e *APIError) Error() string { return "Telegram request failed" }
func (c *Client) post(ctx context.Context, method string, body any, result any) error {
	base := c.BaseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	data, e := json.Marshal(body)
	if e != nil {
		return errors.New("invalid Telegram request")
	}
	r, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/bot"+c.Token+"/"+method, bytes.NewReader(data))
	if e != nil {
		return errors.New("invalid Telegram endpoint")
	}
	r.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, e := client.Do(r)
	if e != nil {
		return errors.New("Telegram connection failed")
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 1024*1024+1)
	data, e = io.ReadAll(limited)
	if e != nil || len(data) > 1024*1024 {
		return errors.New("Telegram response unavailable")
	}
	var envelope struct {
		OK         bool            `json:"ok"`
		Result     json.RawMessage `json:"result"`
		Code       int             `json:"error_code"`
		Parameters struct {
			Retry int `json:"retry_after"`
		} `json:"parameters"`
	}
	if e = json.Unmarshal(data, &envelope); e != nil {
		return errors.New("invalid Telegram response")
	}
	if !envelope.OK || response.StatusCode != 200 {
		return &APIError{Code: envelope.Code, RetryAfter: time.Duration(envelope.Parameters.Retry) * time.Second}
	}
	if result != nil {
		if e = json.Unmarshal(envelope.Result, result); e != nil {
			return errors.New("invalid Telegram result")
		}
	}
	return nil
}
func (c *Client) Verify(ctx context.Context) error {
	var bot struct {
		ID    int64 `json:"id"`
		IsBot bool  `json:"is_bot"`
	}
	if e := c.post(ctx, "getMe", map[string]any{}, &bot); e != nil {
		return e
	}
	if !bot.IsBot || bot.ID == 0 {
		return errors.New("Telegram identity is not a bot")
	}
	var chat struct {
		ID          int64  `json:"id"`
		Type        string `json:"type"`
		Permissions struct {
			CanSend *bool `json:"can_send_messages"`
		} `json:"permissions"`
	}
	if e := c.post(ctx, "getChat", map[string]any{"chat_id": c.ChatID}, &chat); e != nil {
		return e
	}
	var member struct {
		Status  string `json:"status"`
		CanPost *bool  `json:"can_post_messages"`
		CanSend *bool  `json:"can_send_messages"`
	}
	if e := c.post(ctx, "getChatMember", map[string]any{"chat_id": c.ChatID, "user_id": bot.ID}, &member); e != nil {
		return e
	}
	if member.Status == "left" || member.Status == "kicked" || member.CanSend != nil && !*member.CanSend {
		return errors.New("bot cannot send to the configured chat")
	}
	if chat.Type == "channel" && (member.Status != "administrator" || member.CanPost == nil || !*member.CanPost) {
		return errors.New("bot cannot post to the configured channel")
	}
	if member.Status != "administrator" && member.Status != "creator" && chat.Permissions.CanSend != nil && !*chat.Permissions.CanSend {
		return errors.New("chat permissions prohibit bot messages")
	}
	return nil
}
func (c *Client) Send(ctx context.Context, text string) error {
	runes := []rune(text)
	if len(runes) > 3800 {
		text = string(runes[:3800]) + "…"
	}
	return c.post(ctx, "sendMessage", map[string]any{"chat_id": c.ChatID, "text": text, "disable_web_page_preview": true}, nil)
}
