package store

import "testing"

func TestValidateKey(t *testing.T) {
	ok := []string{"FOO", "_X", "A", "Database_URL", "A1"}
	for _, k := range ok {
		if err := ValidateKey(k); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
	}
	bad := []string{"", "1FOO", "FOO-BAR", "FOO.BAR", "FOO BAR", string(make([]byte, 257))}
	for _, k := range bad {
		if err := ValidateKey(k); err == nil {
			t.Fatalf("expected error for %q", k)
		}
	}
}

func TestValidateEnvName(t *testing.T) {
	if err := ValidateEnvName("development"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvName("staging-2"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvName("1x"); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateEnvName(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateValueSize(t *testing.T) {
	if err := ValidateValue(""); err != nil {
		t.Fatal(err)
	}
	ok := make([]byte, MaxValueBytes)
	for i := range ok {
		ok[i] = 'a'
	}
	if err := ValidateValue(string(ok)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateValue(string(append(ok, 'a'))); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateValue("hello\x00"); err == nil {
		t.Fatal("expected reject NUL / non-text")
	}
}
