package store

import "time"

const (
	Magic   = "HUSH1"
	Version = 1
	KeySize = 32
)

type Secret struct {
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Environment struct {
	UpdatedAt time.Time         `json:"updated_at"`
	Secrets   map[string]Secret `json:"secrets"`
}

type Document struct {
	Version      int                     `json:"version"`
	ProjectID    string                  `json:"project_id"`
	Name         string                  `json:"name"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
	Environments map[string]*Environment `json:"environments"`
}

func NewDocument(projectID, name string, now time.Time) *Document {
	now = now.UTC()
	return &Document{
		Version:   Version,
		ProjectID: projectID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		Environments: map[string]*Environment{
			"development": {
				UpdatedAt: now,
				Secrets:   map[string]Secret{},
			},
		},
	}
}
