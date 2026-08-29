//go:build windows

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type fileLock struct {
	file       *os.File
	overlapped windows.Overlapped
}

func acquireFileLock(path string) (*fileLock, error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}
	lock := &fileLock{file: f}
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		1,
		0,
		&lock.overlapped,
	)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return lock, nil
}

func (l *fileLock) Close() error {
	return errors.Join(
		windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &l.overlapped),
		l.file.Close(),
	)
}
