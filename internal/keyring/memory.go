package keyring

type Memory struct {
	m map[string]string
}

func NewMemory() *Memory {
	return &Memory{m: map[string]string{}}
}

func key(service, account string) string { return service + "\x00" + account }

func (m *Memory) Get(service, account string) (string, error) {
	v, ok := m.m[key(service, account)]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *Memory) Set(service, account, secret string) error {
	m.m[key(service, account)] = secret
	return nil
}

func (m *Memory) Delete(service, account string) error {
	delete(m.m, key(service, account))
	return nil
}
