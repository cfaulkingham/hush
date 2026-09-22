package keyring

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// SealPrefix marks a project key wrapped with a passphrase. A sealed key is
// safe to print, paste into a password manager, or commit nowhere: it only
// opens with the passphrase.
const SealPrefix = "hush_sealed_v1_"

// Argon2id parameters for SealPrefix v1. Fixed so sealed blobs stay portable;
// bump the prefix instead of changing these.
const (
	sealTime    = uint32(3)
	sealMemory  = uint32(64 * 1024) // KiB
	sealThreads = uint8(4)
	sealKeyLen  = uint32(32)
)

const sealSaltLen = 16

var (
	ErrBadSealedKey    = errors.New("invalid sealed key (want hush_sealed_v1_ + base64 blob)")
	ErrWrongPassphrase = errors.New("wrong passphrase for sealed key")
	ErrEmptyPassphrase = errors.New("passphrase must not be empty")
)

// IsSealed reports whether s looks like a passphrase-sealed key.
func IsSealed(s string) bool { return strings.HasPrefix(s, SealPrefix) }

// SealKey wraps a raw project key with a passphrase (Argon2id + XChaCha20-Poly1305).
func SealKey(raw []byte, passphrase string) (string, error) {
	if len(raw) != 32 {
		return "", ErrBadKey
	}
	if passphrase == "" {
		return "", ErrEmptyPassphrase
	}
	salt := make([]byte, sealSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	wk := argon2.IDKey([]byte(passphrase), salt, sealTime, sealMemory, sealThreads, sealKeyLen)
	defer clear(wk)
	aead, err := chacha20poly1305.NewX(wk)
	if err != nil {
		return "", err
	}
	ct := aead.Seal(nil, nonce, raw, []byte(SealPrefix))
	blob := make([]byte, 0, len(salt)+len(nonce)+len(ct))
	blob = append(append(append(blob, salt...), nonce...), ct...)
	return SealPrefix + base64.RawURLEncoding.EncodeToString(blob), nil
}

// UnsealKey opens a passphrase-sealed key. A wrong passphrase reports
// ErrWrongPassphrase; malformed input reports ErrBadSealedKey.
func UnsealKey(s, passphrase string) ([]byte, error) {
	if !IsSealed(s) {
		return nil, ErrBadSealedKey
	}
	blob, err := base64.RawURLEncoding.DecodeString(s[len(SealPrefix):])
	minLen := sealSaltLen + chacha20poly1305.NonceSizeX + 32 + 16
	if err != nil || len(blob) < minLen {
		return nil, ErrBadSealedKey
	}
	salt := blob[:sealSaltLen]
	nonce := blob[sealSaltLen : sealSaltLen+chacha20poly1305.NonceSizeX]
	ct := blob[sealSaltLen+chacha20poly1305.NonceSizeX:]
	wk := argon2.IDKey([]byte(passphrase), salt, sealTime, sealMemory, sealThreads, sealKeyLen)
	defer clear(wk)
	aead, err := chacha20poly1305.NewX(wk)
	if err != nil {
		return nil, err
	}
	raw, err := aead.Open(nil, nonce, ct, []byte(SealPrefix))
	if err != nil {
		return nil, ErrWrongPassphrase
	}
	return raw, nil
}
