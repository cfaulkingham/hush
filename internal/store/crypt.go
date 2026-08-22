package store

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

var (
	ErrBadMagic   = errors.New("unrecognized hush store (magic). This CLI reads HUSH1 only.")
	ErrBadKeySize = errors.New("project key must be 32 bytes")
	ErrDecrypt    = errors.New("could not decrypt .hush/store (wrong key or corrupted file)")
	ErrVersion    = errors.New("unsupported hush store version")
)

func Encrypt(doc *Document, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrBadKeySize
	}
	plain, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plain, []byte(Magic))
	out := make([]byte, 0, 5+len(nonce)+len(ct))
	out = append(out, Magic...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

func Decrypt(blob []byte, key []byte) (*Document, error) {
	if len(key) != KeySize {
		return nil, ErrBadKeySize
	}
	if len(blob) < 5+chacha20poly1305.NonceSizeX+16 {
		return nil, ErrDecrypt
	}
	if string(blob[:5]) != Magic {
		return nil, fmt.Errorf("%w: got %q", ErrBadMagic, blob[:min(5, len(blob))])
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := blob[5 : 5+chacha20poly1305.NonceSizeX]
	ct := blob[5+chacha20poly1305.NonceSizeX:]
	plain, err := aead.Open(nil, nonce, ct, []byte(Magic))
	if err != nil {
		return nil, ErrDecrypt
	}
	var doc Document
	if err := json.Unmarshal(plain, &doc); err != nil {
		return nil, ErrDecrypt
	}
	if doc.Version != Version {
		return nil, fmt.Errorf("%w: %d", ErrVersion, doc.Version)
	}
	if doc.Environments == nil {
		doc.Environments = map[string]*Environment{}
	}
	for name, env := range doc.Environments {
		if env == nil {
			doc.Environments[name] = &Environment{Secrets: map[string]Secret{}}
			continue
		}
		if env.Secrets == nil {
			env.Secrets = map[string]Secret{}
		}
	}
	return &doc, nil
}
