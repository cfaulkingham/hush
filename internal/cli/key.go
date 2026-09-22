package cli

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
)

func (a *App) keyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "key", Short: "Backup, restore, or rotate the project key"}
	cmd.AddCommand(a.keyBackupCmd(), a.keyRestoreCmd(), a.keyRotateCmd())
	return cmd
}

func (a *App) keyBackupCmd() *cobra.Command {
	var wrap bool
	var passphraseFile string
	cmd := &cobra.Command{
		Use:   "backup",
		Args:  cobra.NoArgs,
		Short: "Print the project key for backup",
		Long: "Prints the project key.\n\n" +
			"With --wrap the key is sealed with a passphrase first, so the printed\n" +
			"blob is useless without the passphrase and is safer to paste into a\n" +
			"password manager or terminal scrollback.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, key, _, err := a.loadStore()
			if err != nil {
				return err
			}
			if wrap {
				pass, err := a.readPassphrase(passphraseFile, passphraseFile == "")
				if err != nil {
					return err
				}
				s, err := keyring.SealKey(key, pass)
				if err != nil {
					return err
				}
				fmt.Fprintln(a.Stdout, s)
				fmt.Fprintln(a.Stderr, "This key is sealed with your passphrase. Anyone with it and the passphrase can decrypt .hush/store.")
				return nil
			}
			s, err := keyring.FormatKey(key)
			if err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, s)
			fmt.Fprintln(a.Stderr, "This is the project key. Anyone with it can decrypt .hush/store.")
			fmt.Fprintln(a.Stderr, "Prefer `hush key backup --wrap`, which seals the key with a passphrase before printing.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&wrap, "wrap", false, "seal the key with a passphrase before printing")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "read the passphrase from a file instead of prompting")
	return cmd
}

func (a *App) keyRestoreCmd() *cobra.Command {
	var file, passphraseFile string
	noKeyArgs := func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return errors.New("keys are not accepted as arguments: argv is visible in process listings and shell history. Pipe the key on stdin or pass --file")
		}
		return nil
	}
	cmd := &cobra.Command{
		Use:   "restore",
		Args:  noKeyArgs,
		Short: "Restore the project key from stdin, a file, or a hidden prompt",
		Long: "Reads the project key from stdin, --file, or a hidden terminal prompt.\n" +
			"Accepts raw keys (hush_key_v1_...) and sealed keys (hush_sealed_v1_...,\n" +
			"from `hush key backup --wrap`). The key is deliberately not accepted as a\n" +
			"command-line argument: argv is visible in process listings and shell history.\n\n" +
			"  hush key restore < key.txt\n  cat key.txt | hush key restore",
		RunE: func(cmd *cobra.Command, args []string) error {
			material, err := a.readKeyMaterial(file)
			if err != nil {
				return err
			}
			raw, err := a.keyFromMaterial(material, passphraseFile)
			if err != nil {
				return err
			}
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, err := project.Find(cwd)
			if err != nil {
				return err
			}
			if _, err := loadProjectDocument(p, raw); err != nil {
				return err
			}
			if a.hushKey() != "" {
				fmt.Fprintln(a.Stderr, "HUSH_KEY is set; not writing keychain")
				return nil
			}
			s, err := keyring.FormatKey(raw)
			if err != nil {
				return err
			}
			if err := a.Ring.Set(keyring.Service, p.Config.ProjectID, s); err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, "Key restored. Store decrypts OK.")
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "read the key from a file instead of stdin")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "read the passphrase from a file instead of prompting (sealed keys)")
	return cmd
}

func (a *App) keyRotateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rotate",
		Args:  cobra.NoArgs,
		Short: "Re-encrypt the store with a fresh project key",
		Long: "Generates a new project key, re-encrypts .hush/store with it, and stores\n" +
			"the new key in the keychain. After rotation, older `hush key backup`\n" +
			"copies no longer decrypt the store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.hushKey() != "" {
				return errors.New("HUSH_KEY is set; unset it before rotating (rotation installs the new key in the keychain)")
			}
			_, p, oldKey, err := a.projectKey()
			if err != nil {
				return err
			}
			if _, err := loadProjectDocument(p, oldKey); err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintln(a.Stdout, "would generate a new project key, re-encrypt .hush/store, and update the keychain; nothing written.")
				return nil
			}
			raw := make([]byte, store.KeySize)
			if _, err := rand.Read(raw); err != nil {
				return err
			}
			path := project.StorePath(p.Root)
			if err := store.Rekey(path, oldKey, raw); err != nil {
				return err
			}
			s, err := keyring.FormatKey(raw)
			if err != nil {
				return err
			}
			if err := a.Ring.Set(keyring.Service, p.Config.ProjectID, s); err != nil {
				if rbErr := store.Rekey(path, raw, oldKey); rbErr != nil {
					// The store is unreachable via the keychain now. Losing the
					// secrets is worse than exposing the fresh key once: print it
					// so the operator can `hush key restore` it.
					fmt.Fprintf(a.Stderr, "emergency: keychain write and rollback both failed; store now needs this key:\n%s\n", s)
					return errors.Join(err, rbErr)
				}
				return err
			}
			fmt.Fprintln(a.Stdout, "Key rotated. Previous key backups no longer decrypt .hush/store.")
			fmt.Fprintln(a.Stdout, "Next: hush key backup --wrap")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen without writing")
	return cmd
}
