package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

func ColorOn(plain bool) bool {
	if plain || os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func Bold(s string, on bool) string {
	if !on {
		return s
	}
	return lipgloss.NewStyle().Bold(true).Render(s)
}
