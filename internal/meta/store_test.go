package meta_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorae/internal/meta"
)

func TestRecordOpenedAndList(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "meta.db")

	store, err := meta.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()

	first := "/tmp/a.pdf"
	second := "/tmp/b.pdf"

	if err := store.RecordOpened(ctx, first, time.Unix(1000, 0)); err != nil {
		t.Fatalf("record first: %v", err)
	}
	if err := store.RecordOpened(ctx, second, time.Unix(2000, 0)); err != nil {
		t.Fatalf("record second: %v", err)
	}

	results, err := store.ListRecentlyOpened(ctx, 10)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Path != second {
		t.Fatalf("expected most recent to be %s, got %s", second, results[0].Path)
	}

	// Update first to be most recent.
	if err := store.RecordOpened(ctx, first, time.Unix(3000, 0)); err != nil {
		t.Fatalf("record update: %v", err)
	}
	results, err = store.ListRecentlyOpened(ctx, 10)
	if err != nil {
		t.Fatalf("list recent 2: %v", err)
	}
	if results[0].Path != first {
		t.Fatalf("expected updated path first, got %s", results[0].Path)
	}
	if results[0].LastOpenedAt.IsZero() {
		t.Fatalf("expected last opened timestamp to be set")
	}
}
