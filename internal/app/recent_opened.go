package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorae/internal/meta"
)

func (m *Model) recordRecentlyOpened(path string) {
	if path == "" {
		return
	}
	canonical := canonicalPath(path)
	if canonical == "" {
		return
	}
	now := time.Now()
	if m.meta != nil {
		ctx := context.Background()
		if err := m.meta.RecordOpened(ctx, canonical, now); err != nil {
			m.setStatus("Recently read update failed: " + err.Error())
		}
	}
	if m.meta == nil || m.recentlyOpenedDir == "" || m.recentlyOpenedLimit <= 0 {
		return
	}
	if err := rebuildRecentlyOpenedDirectory(m.recentlyOpenedDir, m.recentlyOpenedLimit, m.meta); err != nil {
		m.setStatus("Recently read directory sync failed: " + err.Error())
	}
}

func rebuildRecentlyOpenedDirectory(dest string, limit int, store *meta.Store) error {
	if dest == "" || limit <= 0 || store == nil {
		return nil
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return err
	}

	ctx := context.Background()
	list, err := store.ListRecentlyOpened(ctx, limit)
	if err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(destAbs)
	if err == nil {
		for _, entry := range dirEntries {
			_ = os.RemoveAll(filepath.Join(destAbs, entry.Name()))
		}
	}

	for _, md := range list {
		target := strings.TrimSpace(md.Path)
		if target == "" {
			continue
		}
		openedAt := md.LastOpenedAt
		if openedAt.IsZero() {
			openedAt = time.Now()
		}
		linkName := recentLinkName(filepath.Base(target), md.Title, md.Year, openedAt)
		linkPath := filepath.Join(destAbs, linkName)
		relTarget, err := filepath.Rel(filepath.Dir(linkPath), target)
		if err != nil {
			relTarget = target
		}
		if err := os.Symlink(relTarget, linkPath); err != nil {
			return fmt.Errorf("creating recently opened link for %s: %w", target, err)
		}
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
