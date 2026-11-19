// SPDX-License-Identifier: Elastic-2.0
package library

import (
	"context"
	"errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"io/fs"
	"net"
	"os"
	"strconv"
	"time"
)

type SSHConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	Password       string `json:"-"`
	KeyFile        string `json:"key_file"`
	KnownHostsFile string `json:"known_hosts_file"`
}
type SFTPReader struct {
	client *sftp.Client
	ssh    *ssh.Client
	stop   func() bool
}

func dialSSH(ctx context.Context, c SSHConfig) (*ssh.Client, func() bool, error) {
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Host == "" || c.User == "" || c.Port < 1 || c.Port > 65535 {
		return nil, nil, errors.New("invalid SSH connection settings")
	}
	if (c.KeyFile == "") == (c.Password == "") {
		return nil, nil, errors.New("choose one SSH authentication method")
	}
	var auth ssh.AuthMethod
	if c.Password != "" {
		auth = ssh.Password(c.Password)
	} else {
		key, err := os.ReadFile(c.KeyFile)
		if err != nil {
			return nil, nil, errors.New("SSH key unavailable")
		}
		signer, err := ssh.ParsePrivateKey(key)
		clear(key)
		if err != nil {
			return nil, nil, errors.New("SSH key could not be loaded; provide an unencrypted private key")
		}
		auth = ssh.PublicKeys(signer)
	}
	hostkey, e := knownhosts.New(c.KnownHostsFile)
	if e != nil {
		return nil, nil, errors.New("known-host verification configuration unavailable")
	}
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", addr)
	if e != nil {
		return nil, nil, errors.New("SSH host unreachable")
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	sc, ch, req, e := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: c.User, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: hostkey, Timeout: 10 * time.Second})
	if e != nil {
		stop()
		conn.Close()
		return nil, nil, errors.New("SSH authentication or host-key verification failed")
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(sc, ch, req), stop, nil
}

func Connect(ctx context.Context, c SSHConfig) (*SFTPReader, error) {
	client, stop, e := dialSSH(ctx, c)
	if e != nil {
		return nil, e
	}
	s, e := sftp.NewClient(client, sftp.MaxConcurrentRequestsPerFile(1))
	if e != nil {
		stop()
		client.Close()
		return nil, errors.New("SFTP subsystem unavailable")
	}
	return &SFTPReader{s, client, stop}, nil
}
func (s *SFTPReader) Close() error { s.stop(); e := s.client.Close(); _ = s.ssh.Close(); return e }
func (s *SFTPReader) ReadDir(ctx context.Context, p string) ([]fs.FileInfo, error) {
	return s.client.ReadDirContext(ctx, p)
}
func (s *SFTPReader) Lstat(ctx context.Context, p string) (fs.FileInfo, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return s.client.Lstat(p)
}
func (s *SFTPReader) RealPath(ctx context.Context, p string) (string, error) {
	if e := ctx.Err(); e != nil {
		return "", e
	}
	return s.client.RealPath(p)
}
func (s *SFTPReader) Open(ctx context.Context, p string) (io.ReadCloser, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return s.client.Open(p)
}
