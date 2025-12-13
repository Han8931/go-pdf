package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	targetAbs = canonicalPath(targetAbs)

	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(destAbs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

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
			_ = os.Remove(linkPath)
		}
	}

	title, year := lookupMetadataLabels(store, targetAbs)
	linkName := recentLinkName(filepath.Base(targetAbs), title, year, time.Now())
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
		ts      time.Time
		hasTS   bool
		modTime time.Time
	}
	links := make([]linkInfo, 0, len(dirEntries))
	for _, entry := range dirEntries {
		path := filepath.Join(dest, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		ts, ok := parseRecentLinkTimestamp(entry.Name())
		links = append(links, linkInfo{
			name:    entry.Name(),
			ts:      ts,
			hasTS:   ok,
			modTime: info.ModTime(),
		})
	}
	if len(links) <= limit {
		return nil
	}

	sort.Slice(links, func(i, j int) bool {
		if links[i].hasTS && links[j].hasTS {
			if !links[i].ts.Equal(links[j].ts) {
				return links[i].ts.Before(links[j].ts)
			}
			return links[i].name < links[j].name
		}
		if links[i].hasTS != links[j].hasTS {
			return !links[i].hasTS
		}
		return links[i].modTime.Before(links[j].modTime)
	})

	excess := len(links) - limit
	for i := 0; i < excess; i++ {
		_ = os.Remove(filepath.Join(dest, links[i].name))
	}
	return nil
}

const recentLinkTimestampLayout = "20060102T150405.000000000Z"

func recentLinkName(baseName, title, year string, openedAt time.Time) string {
	base := buildLinkBase(baseName, title, year)
	ts := openedAt.UTC().Format(recentLinkTimestampLayout)
	return fmt.Sprintf("%s-%s", ts, base)
}

func parseRecentLinkTimestamp(name string) (time.Time, bool) {
	idx := strings.IndexByte(name, '-')
	if idx <= 0 {
		return time.Time{}, false
	}
	tsStr := name[:idx]
	ts, err := time.Parse(recentLinkTimestampLayout, tsStr)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func stripRecentLinkPrefix(name string) string {
	idx := strings.IndexByte(name, '-')
	if idx <= 0 {
		return name
	}
	if _, ok := parseRecentLinkTimestamp(name); !ok {
		return name
	}
	trimmed := name[idx+1:]
	if trimmed == "" {
		return name
	}
	return trimmed
}
