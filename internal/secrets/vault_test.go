package secrets

import (
	"bytes"
	"testing"
)

func TestEncryptedCredentials(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	cipher, e := Seal(key, []byte("private-test-token"))
	if e != nil || bytes.Contains(cipher, []byte("private-test-token")) {
		t.Fatal("credential not encrypted")
	}
	plain, e := Open(key, cipher)
	if e != nil || string(plain) != "private-test-token" {
		t.Fatal("roundtrip failed")
	}
	cipher[len(cipher)-1] ^= 1
	if _, e = Open(key, cipher); e == nil {
		t.Fatal("tamper accepted")
	}
	if _, e = Open(bytes.Repeat([]byte{2}, 32), cipher); e == nil {
		t.Fatal("wrong key accepted")
	}
	if _, e = Seal([]byte("short"), []byte("token")); e == nil {
		t.Fatal("short key accepted")
	}
}
