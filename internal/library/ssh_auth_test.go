// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHAndSFTPReadersAuthenticate(t *testing.T) {
	work := t.TempDir()
	root := filepath.Join(work, "films")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Film (2024).mkv"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	hostKey := newTestSSHKey(t)
	userKey := newTestSSHKey(t)
	signer, err := ssh.NewSignerFromKey(userKey)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().(*net.TCPAddr)
	known := filepath.Join(work, "known_hosts")
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(known, []byte(knownhosts.Line([]string{addr.String()}, hostSigner.PublicKey())+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	keyPEM, err := ssh.MarshalPrivateKey(userKey, "test")
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(work, "client-key")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(keyPEM), 0600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, password, key string
		shell               bool
	}{
		{name: "sftp-password", password: "demo password"},
		{name: "sftp-key", key: keyFile},
		{name: "shell-password", password: "demo password", shell: true},
		{name: "shell-key", key: keyFile, shell: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.shell {
				if _, err := exec.LookPath("python3"); err != nil {
					t.Skip("SSH shell adapter requires Python 3")
				}
			}
			config := &ssh.ServerConfig{}
			config.AddHostKey(hostSigner)
			if tc.password != "" {
				config.PasswordCallback = func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
					if c.User() != "reader" || string(password) != tc.password {
						return nil, errors.New("invalid test password")
					}
					return nil, nil
				}
			} else {
				config.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
					if c.User() != "reader" || !bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
						return nil, errors.New("invalid test key")
					}
					return nil, nil
				}
			}
			done := make(chan error, 1)
			if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				done <- serveTestSSH(conn, config, tc.shell)
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cfg := SSHConfig{Host: addr.IP.String(), Port: addr.Port, User: "reader", Password: tc.password, KeyFile: tc.key, KnownHostsFile: known}
			var reader interface {
				Reader
				Close() error
			}
			if tc.shell {
				reader, err = ConnectShell(ctx, cfg, root)
			} else {
				reader, err = Connect(ctx, cfg)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			items, err := reader.ReadDir(ctx, root)
			if err != nil || len(items) != 1 || items[0].Name() != "Film (2024).mkv" {
				t.Fatalf("expected media listing, got %v (%v)", items, err)
			}
			_ = reader.Close()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func newTestSSHKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func serveTestSSH(conn net.Conn, cfg *ssh.ServerConfig, shell bool) error {
	session, channels, requests, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return err
	}
	defer session.Close()
	go ssh.DiscardRequests(requests)
	for req := range channels {
		ch, incoming, err := req.Accept()
		if err != nil {
			return err
		}
		for request := range incoming {
			switch {
			case shell && request.Type == "exec":
				var payload struct{ Command string }
				if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
					return err
				}
				_ = request.Reply(true, nil)
				command := exec.Command("sh", "-c", payload.Command)
				command.Stdout = ch
				var exit uint32
				if command.Run() != nil {
					exit = 1
				}
				_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{exit}))
			case !shell && request.Type == "subsystem":
				_ = request.Reply(true, nil)
				server, err := sftp.NewServer(ch, sftp.ReadOnly())
				if err != nil {
					return err
				}
				err = server.Serve()
				_ = server.Close()
				if !errors.Is(err, io.EOF) {
					return err
				}
			default:
				_ = request.Reply(false, nil)
			}
			_ = ch.Close()
			break
		}
	}
	return nil
}
