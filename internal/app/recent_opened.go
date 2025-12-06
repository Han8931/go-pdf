package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gorae/internal/meta"
)

func (m *Model) recordRecentlyOpened(path string) {
	if path == "" || m.recentlyOpenedDir == "" || m.recentlyOpenedLimit <= 0 {
		return
	}
	if err := updateRecentlyOpenedDirectory(m.recentlyOpenedDir, path, m.recentlyOpenedLimit, m.meta); err != nil {
		m.setStatus("Recently opened update failed: " + err.Error())
	}
}

func updateRecentlyOpenedDirectory(dest, openedPath string, limit int, store *meta.Store) error {
	if dest == "" || openedPath == "" || limit <= 0 {
		return nil
	}

	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(openedPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(destAbs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	var existingLink string
	for _, entry := range dirEntries {
		linkPath := filepath.Join(destAbs, entry.Name())
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
			target = filepath.Join(destAbs, target)
		}
		if filepath.Clean(target) == filepath.Clean(targetAbs) {
			existingLink = linkPath
			break
		}
	}

	if existingLink != "" {
		now := time.Now()
		_ = os.Chtimes(existingLink, now, now)
		return trimRecentlyOpened(destAbs, limit)
	}

	title, year := lookupMetadataLabels(store, targetAbs)
	linkName, err := nextAvailableLinkName(destAbs, filepath.Base(targetAbs), title, year)
	if err != nil {
		return err
	}
	linkPath := filepath.Join(destAbs, linkName)
	relTarget, err := filepath.Rel(filepath.Dir(linkPath), targetAbs)
	if err != nil {
		relTarget = targetAbs
	}
	if err := os.Symlink(relTarget, linkPath); err != nil {
		return fmt.Errorf("creating recently opened link for %s: %w", targetAbs, err)
	}

	return trimRecentlyOpened(destAbs, limit)
}

func trimRecentlyOpened(dest string, limit int) error {
	dirEntries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}

	type linkInfo struct {
		name    string
		modTime time.Time
	}
	var links []linkInfo
	for _, entry := range dirEntries {
		path := filepath.Join(dest, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		links = append(links, linkInfo{name: entry.Name(), modTime: info.ModTime()})
	}
	if len(links) <= limit {
		return nil
	}
	sort.Slice(links, func(i, j int) bool {
		return links[i].modTime.Before(links[j].modTime)
	})
	excess := len(links) - limit
	for i := 0; i < excess; i++ {
		_ = os.Remove(filepath.Join(dest, links[i].name))
	}
	return nil
}

func nextAvailableLinkName(dir, baseName, title, year string) (string, error) {
	base := buildLinkBase(baseName, title, year)
	name := base
	suffix := 2
	for {
		path := filepath.Join(dir, name)
		_, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
		name = appendNumericSuffix(base, suffix)
		suffix++
	}
}
