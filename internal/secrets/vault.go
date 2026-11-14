// SPDX-License-Identifier: Elastic-2.0
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

func vault(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("connection encryption key must be 32 bytes")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, errors.New("encryption unavailable")
	}
	return cipher.NewGCM(block)
}
func Seal(key, plain []byte) ([]byte, error) {
	g, e := vault(key)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, errors.New("encryption randomness unavailable")
	}
	out := append([]byte{1}, nonce...)
	return g.Seal(out, nonce, plain, []byte("lookout-connection-v1")), nil
}
func Open(key, sealed []byte) ([]byte, error) {
	g, e := vault(key)
	if e != nil {
		return nil, e
	}
	if len(sealed) < 1+g.NonceSize()+g.Overhead() || sealed[0] != 1 {
		return nil, errors.New("invalid encrypted connection")
	}
	plain, e := g.Open(nil, sealed[1:1+g.NonceSize()], sealed[1+g.NonceSize():], []byte("lookout-connection-v1"))
	if e != nil {
		return nil, errors.New("connection could not be decrypted")
	}
	return plain, nil
}
