package store

import (
	"errors"
	"os"

	"github.com/cfaulkingham/hush/internal/safeio"
)

var (
	ErrSymlink     = errors.New("refusing to write through symlink .hush/store")
	ErrLockSymlink = errors.New("refusing to lock through symlink .hush/store.lock")
)

func openLockFile(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return nil, ErrLockSymlink
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func WriteFile(path string, blob []byte) error {
	err := safeio.WriteFile(path, blob, 0600)
	if errors.Is(err, safeio.ErrSymlink) {
		return ErrSymlink
	}
	return err
}

func Save(path string, doc *Document, key []byte) error {
	blob, err := Encrypt(doc, key)
	if err != nil {
		return err
	}
	return WriteFile(path, blob)
}

func Load(path string, key []byte) (*Document, error) {
	blob, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decrypt(blob, key)
}

// Update serializes read-modify-write transactions across hush processes.
// The callback runs while the store lock is held and must not call Update.
func Update(path string, key []byte, update func(*Document) error) (err error) {
	lock, err := acquireFileLock(path + ".lock")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, lock.Close())
	}()

	doc, err := Load(path, key)
	if err != nil {
		return err
	}
	if err := update(doc); err != nil {
		return err
	}
	return Save(path, doc, key)
}
