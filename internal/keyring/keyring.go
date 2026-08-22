package keyring

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

const (
	Service = "hush"
	Prefix  = "hush_key_v1_"
)

var (
	ErrNoKey    = errors.New("no project key. Restore with hush key restore <key>, or set HUSH_KEY.")
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

func Resolve(r Ring, projectID string) ([]byte, error) {
	if s := os.Getenv("HUSH_KEY"); s != "" {
		return ParseKey(s)
	}
	if r == nil {
		return nil, ErrNoKey
	}
	s, err := r.Get(Service, projectID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNoKey
		}
		return nil, errors.Join(ErrBackend, err)
	}
	return ParseKey(s)
}
