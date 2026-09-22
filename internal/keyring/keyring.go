package keyring

import (
	"encoding/hex"
	"errors"
	"strings"
)

const (
	Service = "hush"
	Prefix  = "hush_key_v1_"

	SourceEnv     = "HUSH_KEY"
	SourceKeyring = "keychain"
)

var (
	ErrNoKey    = errors.New("no project key. Restore one with `hush key restore` (reads the key from stdin), or set HUSH_KEY.")
	ErrNotFound = errors.New("not found")
	ErrBadKey   = errors.New("invalid project key (want hush_key_v1_ + 64 hex chars)")
	ErrBackend  = errors.New("keychain backend failure")
)

type Ring interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

func FormatKey(raw []byte) (string, error) {
	if len(raw) != 32 {
		return "", ErrBadKey
	}
	return Prefix + hex.EncodeToString(raw), nil
}

func ParseKey(s string) ([]byte, error) {
	if !strings.HasPrefix(s, Prefix) {
		return nil, ErrBadKey
	}
	body := s[len(Prefix):]
	raw, err := hex.DecodeString(body)
	if err != nil || len(raw) != 32 {
		return nil, ErrBadKey
	}
	return raw, nil
}

// Resolve returns the project key and where it came from. A non-empty hushKey
// (the HUSH_KEY override, supplied by the caller) wins over the keychain
// (intentional: CI and "try this backup key").
func Resolve(r Ring, projectID, hushKey string) ([]byte, string, error) {
	if hushKey != "" {
		raw, err := ParseKey(hushKey)
		return raw, SourceEnv, err
	}
	if r == nil {
		return nil, SourceKeyring, ErrNoKey
	}
	s, err := r.Get(Service, projectID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, SourceKeyring, ErrNoKey
		}
		return nil, SourceKeyring, errors.Join(ErrBackend, err)
	}
	raw, err := ParseKey(s)
	return raw, SourceKeyring, err
}
