package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitignoreCoversHush(t *testing.T) {
	cases := []struct {
		content string
		want    bool
	}{
		{"", false},
		{".hush/\n", true},
		{".hush\n", true},
		{"/.hush/\n", true},
		{".hush/**\n", true},
		{"**/.hush/\n", true},
		{"# .hush/\n", false},
		{"node_modules\n.hush\n", true},
		{"*.env\n", false},
		{".hush\n!.hush\n", false},
		{"!.hush\n.hush\n", true},
		{"  .hush/  \n", true},
	}
	for _, tc := range cases {
		if got := gitignoreCoversHush(tc.content); got != tc.want {
			t.Fatalf("covers(%q) = %v want %v", tc.content, got, tc.want)
		}
	}
}

func TestEnsureGitignoreFallbackVariants(t *testing.T) {
	unknown := func(string) gitVerdict { return gitUnknown }
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".hush\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitignore(dir, unknown); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Contains(string(b), ".hush/") {
		t.Fatalf("appended duplicate for covered variant: %q", b)
	}
}

func TestEnsureGitignoreAppendsOnce(t *testing.T) {
	unknown := func(string) gitVerdict { return gitUnknown }
	dir := t.TempDir()
	if err := ensureGitignore(dir, unknown); err != nil {
		t.Fatal(err)
	}
	if err := ensureGitignore(dir, unknown); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(b), ".hush/") != 1 {
		t.Fatalf("duplicate entries: %q", b)
	}
}

func TestEnsureGitignoreDefersToGit(t *testing.T) {
	dir := t.TempDir()
	ignored := func(string) gitVerdict { return gitIgnored }
	if err := ensureGitignore(dir, ignored); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("touched .gitignore although git already ignores .hush (parent coverage)")
	}
}

func TestEnsureGitignoreVerifiesWithGit(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	verdict := func(string) gitVerdict {
		calls++
		if calls == 1 {
			return gitNotIgnored
		}
		return gitIgnored // post-append check
	}
	if err := ensureGitignore(dir, verdict); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(b), ".hush/") {
		t.Fatalf("did not append: %q", b)
	}
}

func TestEnsureGitignoreFailsWhenGitStillTracks(t *testing.T) {
	dir := t.TempDir()
	stuck := func(string) gitVerdict { return gitNotIgnored }
	err := ensureGitignore(dir, stuck)
	if err == nil || !strings.Contains(err.Error(), "git would still track") {
		t.Fatalf("%v", err)
	}
}
