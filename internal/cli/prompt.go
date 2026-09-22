package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cfaulkingham/hush/internal/keyring"
	"golang.org/x/term"
)

// ErrNotInteractive marks prompts that cannot be shown without a terminal.
var ErrNotInteractive = errors.New("no interactive terminal")

// askTerminal reads one visible line from the terminal. It is the default
// App.Ask used by NewOSApp.
func askTerminal(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", ErrNotInteractive
	}
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// confirm asks yes/no. Prompting is best-effort for humans: scripts and
// non-interactive sessions proceed (use --dry-run to preview, --yes to skip
// the question on a terminal).
func (a *App) confirm(prompt string, assumeYes bool) (bool, error) {
	if assumeYes || a.Ask == nil {
		return true, nil
	}
	answer, err := a.Ask(prompt)
	if errors.Is(err, ErrNotInteractive) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// readTerminalSecret reads one secret from the terminal without echo. It is
// the default App.ReadSecret used by NewOSApp.
func readTerminalSecret(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("no interactive terminal for hidden input")
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *App) readHidden(prompt string) (string, error) {
	if a.ReadSecret == nil {
		return "", errors.New("no interactive terminal for hidden input")
	}
	return a.ReadSecret(prompt)
}

// readPassphrase obtains a passphrase from --passphrase-file or a hidden
// terminal prompt. Prompts twice and compares when confirm is set.
func (a *App) readPassphrase(passphraseFile string, confirm bool) (string, error) {
	if passphraseFile != "" {
		b, err := os.ReadFile(passphraseFile)
		if err != nil {
			return "", fmt.Errorf("cannot read %s: %w", passphraseFile, err)
		}
		s := strings.TrimRight(string(b), "\r\n")
		if s == "" {
			return "", keyring.ErrEmptyPassphrase
		}
		return s, nil
	}
	first, err := a.readHidden("Passphrase: ")
	if err != nil {
		return "", err
	}
	if first == "" {
		return "", keyring.ErrEmptyPassphrase
	}
	if confirm {
		second, err := a.readHidden("Confirm passphrase: ")
		if err != nil {
			return "", err
		}
		if first != second {
			return "", errors.New("passphrases do not match")
		}
	}
	return first, nil
}

// readKeyMaterial reads a key (raw or sealed) from --file, stdin, or a hidden
// terminal prompt. It is never read from command-line arguments: argv leaks
// into process listings and shell history.
func (a *App) readKeyMaterial(file string) (string, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("cannot read %s: %w", file, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if f, ok := a.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		s, err := a.readHidden("Project key (hidden): ")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(s), nil
	}
	b, err := io.ReadAll(a.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// keyFromMaterial parses key material, unsealing it first when it was
// passphrase-wrapped.
func (a *App) keyFromMaterial(material, passphraseFile string) ([]byte, error) {
	if material == "" {
		return nil, errors.New("no key provided (pipe it on stdin or pass --file)")
	}
	if keyring.IsSealed(material) {
		pass, err := a.readPassphrase(passphraseFile, false)
		if err != nil {
			return nil, err
		}
		return keyring.UnsealKey(material, pass)
	}
	return keyring.ParseKey(material)
}
