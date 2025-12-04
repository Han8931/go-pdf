package main

import (
	"flag"
	"log"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"pdf-tui/internal/app"
	"pdf-tui/internal/config"
	"pdf-tui/internal/meta"
)

func main() {
	rootFlag := flag.String("root", "", "Root directory to start in (overrides config watch_dir)")
	flag.Parse()

	cfg, err := config.LoadOrInit()
	if err != nil {
		log.Fatal(err)
	}

	root := cfg.WatchDir
	if *rootFlag != "" {
		root = *rootFlag
	}

	dbPath := filepath.Join(cfg.MetaDir, "metadata.db")
	store, err := meta.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	m := app.NewModel(root, store)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
