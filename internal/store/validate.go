package store

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const MaxValueBytes = 65536

var (
	keyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
)

func ValidateKey(key string) error {
	if len(key) < 1 || len(key) > 256 || !keyRe.MatchString(key) {
		return fmt.Errorf("invalid secret key %q (use KEY_NAME)", key)
	}
	if strings.EqualFold(key, "HUSH_KEY") {
		return fmt.Errorf("invalid secret key %q (reserved by hush)", key)
	}
	return nil
}

func ValidateEnvName(name string) error {
	if len(name) < 1 || len(name) > 64 || !envRe.MatchString(name) {
		return fmt.Errorf("invalid environment name %q", name)
	}
	return nil
}

func ValidateValue(value string) error {
	if len(value) > MaxValueBytes {
		return fmt.Errorf("secret value exceeds %d bytes", MaxValueBytes)
	}
	if !utf8.ValidString(value) || containsNUL(value) {
		return fmt.Errorf("secret value must be UTF-8 text")
	}
	return nil
}

func containsNUL(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}
