package ui

import (
	"io/fs"
)

func IconFor(e fs.DirEntry) string {
	if e.IsDir() {
		return "📁"
	}
	return "📄"
}
