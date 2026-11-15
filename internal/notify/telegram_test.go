package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramVerificationNeverSendsMessages(t *testing.T) {
	methods := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		methods = append(methods, method)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "getMe":
			w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true}}`))
		case "getChat":
			w.Write([]byte(`{"ok":true,"result":{"id":2,"type":"private"}}`))
		case "getChatMember":
			w.Write([]byte(`{"ok":true,"result":{"status":"member"}}`))
		default:
			t.Fatal("verification sent message", method)
		}
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, Token: "123:private-test-token", ChatID: "2", HTTP: server.Client()}
	if e := client.Verify(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(methods) != 3 {
		t.Fatal(methods)
	}
}
func TestTelegramMessageAndErrorsUseMockOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		if body["chat_id"] != "2" || body["text"] != "Genuine release fixture" {
			t.Fatal("wrong message shape")
		}
		if _, ok := body["parse_mode"]; ok {
			t.Fatal("markup parsing unnecessary")
		}
		w.WriteHeader(429)
		w.Write([]byte(`{"ok":false,"error_code":429,"description":"private-test-token must not be logged","parameters":{"retry_after":30}}`))
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, Token: "123:private-test-token", ChatID: "2", HTTP: server.Client()}
	e := client.Send(context.Background(), "Genuine release fixture")
	if e == nil || strings.Contains(e.Error(), "private-test-token") {
		t.Fatal("error exposed credential", e)
	}
}
