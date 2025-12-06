package main

import (
	"flag"
	"log"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gorae/internal/app"
	"gorae/internal/config"
	"gorae/internal/meta"
)

func main() {
	rootFlag := flag.String("root", "", "Root directory to start in (overrides config watch_dir)")
	flag.Parse()

	cfg, err := config.LoadOrInit()
	if err != nil {
		log.Fatal(err)
	}

	origWatch := cfg.WatchDir
	root := cfg.WatchDir
	if *rootFlag != "" {
		root = *rootFlag
	}
	cfg.WatchDir = root
	if *rootFlag != "" {
		defaultOldRecent := filepath.Join(origWatch, "_recent")
		defaultAdded := filepath.Join(origWatch, "_recently_added")
		trimmedRecent := strings.TrimSpace(cfg.RecentlyAddedDir)
		if trimmedRecent == "" || trimmedRecent == defaultOldRecent || trimmedRecent == defaultAdded {
			cfg.RecentlyAddedDir = filepath.Join(root, "_recently_added")
		}

		defaultOpened := filepath.Join(origWatch, "_recently_opened")
		trimmedOpened := strings.TrimSpace(cfg.RecentlyOpenedDir)
		if trimmedOpened == "" || trimmedOpened == defaultOpened {
			cfg.RecentlyOpenedDir = filepath.Join(root, "_recently_opened")
		}
	}

	dbPath := filepath.Join(cfg.MetaDir, "metadata.db")
	store, err := meta.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	m := app.NewModel(cfg, store)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
