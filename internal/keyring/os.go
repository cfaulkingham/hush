package keyring

import (
	"errors"

	oskeyring "github.com/zalando/go-keyring"
)

type OS struct{}

func (OS) Get(service, account string) (string, error) {
	s, err := oskeyring.Get(service, account)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", errors.Join(ErrBackend, err)
	}
	return s, nil
}

func (OS) Set(service, account, secret string) error {
	if err := oskeyring.Set(service, account, secret); err != nil {
		return errors.Join(ErrBackend, err)
	}
	return nil
}

func (OS) Delete(service, account string) error {
	err := oskeyring.Delete(service, account)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrBackend, err)
	}
	return nil
}
