package main

import (
	"flag"
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"pdf-tui/internal/app" // <- module name + path
)

func main() {
	root := flag.String("root", ".", "Root directory to start in")
	flag.Parse()

	m := app.NewModel(*root)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(), // full-screen TUI
	)

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
