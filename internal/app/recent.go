package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pdf-tui/internal/meta"
)

const defaultRecentlyAddedSyncInterval = time.Minute

func (m *Model) maybeSyncRecentlyAddedDir(force bool) error {
	if m.recentlyAddedDir == "" || m.recentlyAddedMaxAge <= 0 {
		return nil
	}
	if !force && !m.lastRecentlyAddedSync.IsZero() {
		if time.Since(m.lastRecentlyAddedSync) < m.recentlyAddedSyncInt {
			return nil
		}
	}
	if err := syncRecentlyAddedDirectory(m.root, m.recentlyAddedDir, m.recentlyAddedMaxAge, m.meta); err != nil {
		return err
	}
	m.lastRecentlyAddedSync = time.Now()
	return nil
}

func syncRecentlyAddedDirectory(root, recentDir string, maxAge time.Duration, store *meta.Store) error {
	if root == "" || recentDir == "" || maxAge <= 0 {
		return nil
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	recentAbs, err := filepath.Abs(recentDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	desired := make(map[string]string)

	err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			return walkErr
		}

		if path == recentAbs {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if path != rootAbs && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}

		base := filepath.Base(rel)
		title, year := lookupMetadataLabels(store, path)
		linkName := mapBackedLinkName(base, title, year, desired)
		desired[linkName] = path
		return nil
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(recentAbs, 0o755); err != nil {
		return err
	}

	existing := make(map[string]string)
	dirEntries, err := os.ReadDir(recentAbs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, entry := range dirEntries {
		linkPath := filepath.Join(recentAbs, entry.Name())
		info, err := os.Lstat(linkPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(recentAbs, target)
		}
		existing[entry.Name()] = filepath.Clean(target)
	}

	for name := range existing {
		target, ok := desired[name]
		if !ok || filepath.Clean(target) != existing[name] {
			_ = os.Remove(filepath.Join(recentAbs, name))
		}
	}

	for name, target := range desired {
		linkPath := filepath.Join(recentAbs, name)
		existingTarget, ok := existing[name]
		if ok && filepath.Clean(existingTarget) == filepath.Clean(target) {
			continue
		}
		_ = os.Remove(linkPath)
		relTarget, err := filepath.Rel(filepath.Dir(linkPath), target)
		if err != nil {
			relTarget = target
		}
		if err := os.Symlink(relTarget, linkPath); err != nil {
			return fmt.Errorf("creating symlink for %s: %w", target, err)
		}
	}

	return nil
}

func mapBackedLinkName(baseName, title, year string, used map[string]string) string {
	base := buildLinkBase(baseName, title, year)
	name := base
	suffix := 2
	for {
		if _, exists := used[name]; !exists {
			return name
		}
		name = appendNumericSuffix(base, suffix)
		suffix++
	}
}

func sanitizeLinkName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "_")
	trimmed = strings.ReplaceAll(trimmed, "/", "_")
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	return trimmed
}

func buildLinkBase(baseName, title, year string) string {
	ext := filepath.Ext(baseName)
	core := strings.TrimSuffix(baseName, ext)
	core = sanitizeLinkName(core)
	if core == "" {
		core = "_"
	}
	title = sanitizeLinkName(title)
	year = sanitizeLinkName(year)
	if title != "" {
		if year == "" {
			year = "-"
		}
		core = fmt.Sprintf("[%s][%s]", year, title)
	}
	return core + ext
}

func appendNumericSuffix(name string, suffix int) string {
	ext := filepath.Ext(name)
	core := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s__%d%s", core, suffix, ext)
}

func lookupMetadataLabels(store *meta.Store, path string) (title, year string) {
	if store == nil || path == "" {
		return "", ""
	}
	ctx := context.Background()
	md, err := store.Get(ctx, canonicalPath(path))
	if err != nil || md == nil {
		return "", ""
	}
	return strings.TrimSpace(md.Title), strings.TrimSpace(md.Year)
}
