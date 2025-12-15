package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectPDFFilesSkipsHelperDirs(t *testing.T) {
	root := t.TempDir()

	mainPDF := filepath.Join(root, "paper.pdf")
	writeDummyPDF(t, mainPDF)

	recentDir := filepath.Join(root, "Recently Added")
	if err := os.MkdirAll(recentDir, 0o755); err != nil {
		t.Fatalf("mkdir recently added: %v", err)
	}
	writeDummyPDF(t, filepath.Join(recentDir, "dup.pdf"))

	files, warnings, err := collectPDFFiles(root, []string{recentDir})
	if err != nil {
		t.Fatalf("collect pdfs: %v", err)
	}
	if len(warnings) > 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(files) != 1 || files[0] != mainPDF {
		t.Fatalf("expected only %s, got %v", mainPDF, files)
	}
}

func TestCollectPDFFilesSkipsMultipleHelperDirs(t *testing.T) {
	root := t.TempDir()
	writeDummyPDF(t, filepath.Join(root, "base.pdf"))

	helpers := []string{
		filepath.Join(root, "Recently Read"),
		filepath.Join(root, "Favorites"),
		filepath.Join(root, "To Read"),
	}
	for _, dir := range helpers {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir helper %s: %v", dir, err)
		}
		writeDummyPDF(t, filepath.Join(dir, "dup.pdf"))
	}

	files, _, err := collectPDFFiles(root, helpers)
	if err != nil {
		t.Fatalf("collect pdfs: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	if files[0] != filepath.Join(root, "base.pdf") {
		t.Fatalf("unexpected file returned: %v", files)
	}
}

func writeDummyPDF(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write pdf %s: %v", path, err)
	}
}
