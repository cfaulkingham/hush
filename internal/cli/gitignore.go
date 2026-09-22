package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ensureGitignore makes sure .hush is ignored by git before project state is
// created. When git can answer for this directory the verdict comes from git
// itself (parent .gitignore files, negations, and exotic patterns included);
// without git it falls back to matching the local .gitignore for common
// spellings of ".hush". The check func is injected so tests stay hermetic.
func ensureGitignore(dir string, check func(string) gitVerdict) error {
	hushPath := filepath.Join(dir, ".hush")
	verdict := check(hushPath)
	if verdict == gitIgnored {
		return nil
	}
	if verdict == gitUnknown {
		b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if gitignoreCoversHush(string(b)) {
			return nil
		}
	}
	if err := appendGitignore(dir); err != nil {
		return err
	}
	if verdict == gitNotIgnored && check(hushPath) == gitNotIgnored {
		return errors.New("git would still track .hush after adding it to .gitignore (is it already committed?); fix that before running hush init")
	}
	return nil
}

func appendGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	add := ".hush/\n"
	if len(b) > 0 && b[len(b)-1] != '\n' {
		add = "\n" + add
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(add)
	return err
}

// gitignoreCoversHush reports whether gitignore-style content already ignores
// ".hush", accepting the common spellings: ".hush", ".hush/", "/.hush",
// ".hush/**", "**/.hush/". A later negation ("!.hush") wins.
func gitignoreCoversHush(content string) bool {
	covered := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negated := strings.HasPrefix(line, "!")
		p := strings.TrimPrefix(line, "!")
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimPrefix(p, "**/")
		p = strings.TrimSuffix(p, "/**")
		p = strings.TrimSuffix(p, "/")
		if p == ".hush" {
			covered = !negated
		}
	}
	return covered
}

type gitVerdict int

const (
	gitUnknown    gitVerdict = iota // not in a git repo, or git unavailable
	gitIgnored                      // path is ignored by git
	gitNotIgnored                   // path would be visible to git
)

// gitCheck reports how git would treat path. It never fails: no git binary
// and no repository both report gitUnknown.
func gitCheck(path string) gitVerdict {
	abs, err := filepath.Abs(path)
	if err != nil {
		return gitUnknown
	}
	cmd := exec.Command("git", "-C", filepath.Dir(abs), "check-ignore", "-q", "--", abs)
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		switch ee.ExitCode() {
		case 0:
			return gitIgnored
		case 1:
			return gitNotIgnored
		}
	}
	return gitUnknown
}

func (a *App) gitCheckPath(path string) gitVerdict {
	if a.GitCheck != nil {
		return a.GitCheck(path)
	}
	return gitCheck(path)
}
