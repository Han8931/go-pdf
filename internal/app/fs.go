package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (m *Model) loadEntries() {
	ents, err := os.ReadDir(m.cwd)
	m.err = err
	if err != nil {
		m.entries = nil
		m.cursor = 0
		return
	}

	// hide dotfiles and non-PDF files (but keep directories)
	filtered := make([]fs.DirEntry, 0, len(ents))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}

		if !e.IsDir() {
			name := strings.ToLower(e.Name())
			if !strings.HasSuffix(name, ".pdf") {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	// sort dirs first then alpha
	sort.SliceStable(filtered, func(i, j int) bool {
		di, dj := filtered[i].IsDir(), filtered[j].IsDir()
		if di != dj {
			return di && !dj
		}
		return strings.ToLower(filtered[i].Name()) <
			strings.ToLower(filtered[j].Name())
	})

	m.entries = filtered

	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
	m.ensureCursorVisible()
	m.refreshEntryTitles()
}

func (m *Model) removeFromCut(path string) {
	out := m.cut[:0]
	for _, c := range m.cut {
		if c != path {
			out = append(out, c)
		}
	}
	m.cut = out
}

func (m Model) selectionOrCurrent() []string {
	if len(m.selected) > 0 {
		out := make([]string, 0, len(m.selected))
		for p := range m.selected {
			out = append(out, p)
		}
		return out
	}
	if len(m.entries) == 0 {
		return nil
	}
	full := filepath.Join(m.cwd, m.entries[m.cursor].Name())
	return []string{full}
}

func avoidNameClash(dst string) string {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}
	ext := filepath.Ext(dst)
	base := strings.TrimSuffix(filepath.Base(dst), ext)
	dir := filepath.Dir(dst)

	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func (m *Model) refreshEntryTitles() {
	if m.entryTitles == nil {
		m.entryTitles = make(map[string]string)
	}
	for k := range m.entryTitles {
		delete(m.entryTitles, k)
	}

	ctx := context.Background()
	for _, e := range m.entries {
		full := filepath.Join(m.cwd, e.Name())
		m.entryTitles[full] = m.resolveEntryTitle(ctx, full, e)
	}
}

func (m *Model) resolveEntryTitle(ctx context.Context, fullPath string, entry fs.DirEntry) string {
	if entry.IsDir() {
		return entry.Name() + "/"
	}

	name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
	if m.meta == nil {
		return name
	}

	md, err := m.meta.Get(ctx, fullPath)
	if err != nil || md == nil {
		return name
	}
	title := strings.TrimSpace(md.Title)
	if title == "" {
		return name
	}
	return title
}
