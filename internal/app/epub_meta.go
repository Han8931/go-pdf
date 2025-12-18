package app

import (
	"fmt"
	"path/filepath"
	"strings"
)

// parseEPUBMetadata parses basic metadata from the EPUB OPF.
func parseEPUBMetadata(path string) (pdfMeta, error) {
	opf, err := extractEPUBPackage(path)
	if err != nil {
		return pdfMeta{}, err
	}
	meta := pdfMeta{
		Title:  strings.TrimSpace(opf.Title),
		Author: strings.TrimSpace(opf.Creator),
		Tag:    strings.TrimSpace(opf.Subject),
		Year:   extractYear(opf.Date),
	}
	return meta, nil
}

// extractEPUBPackage is a helper to get the parsed OPF package for metadata.
func extractEPUBPackage(path string) (opfPackage, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".epub" {
		return opfPackage{}, fmt.Errorf("not an epub")
	}
	rc, pkg, err := openEPUBPackage(path)
	if err != nil {
		return opfPackage{}, err
	}
	defer rc.Close()
	return pkg, nil
}

