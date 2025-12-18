package app

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
	"encoding/xml"
)

// readEPUBText extracts concatenated plain text from an EPUB by following its
// spine and stripping HTML tags. This is a lightweight, best-effort extractor
// for search/snippet generation (not full fidelity rendering).
func readEPUBText(path string) (string, error) {
	r, opf, err := openEPUBPackage(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var buf strings.Builder
	baseDir := filepath.Dir(opf.rootfilePath)
	for _, itemref := range opf.Spine {
		itemPath := opf.Manifest[itemref]
		if itemPath == "" {
			continue
		}
		full := filepath.Clean(filepath.Join(baseDir, itemPath))
		data, err := readZipFile(r.File, full)
		if err != nil {
			continue
		}
		text := stripHTMLToText(data)
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteString("\n\n")
			}
			buf.WriteString(text)
		}
	}

	return buf.String(), nil
}

type opfPackage struct {
	Manifest map[string]string // id -> href
	Spine    []string          // ordered itemref IDs
	Title    string
	Creator  string
	Date     string
	Subject  string
	rootfilePath string
}

// extractEPUBPreview returns up to maxLines of text for preview purposes.
func extractEPUBPreview(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		maxLines = 20
	}
	text, err := readEPUBText(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return []string{"(no text extracted)"}, nil
	}
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		lines = append(lines, l)
		if maxLines > 0 && len(lines) >= maxLines {
			break
		}
	}
	if len(lines) == 0 {
		lines = []string{"(no text extracted)"}
	}
	return lines, nil
}

// readZipFile reads the first file whose name matches target (case-insensitive).
func readZipFile(files []*zip.File, target string) ([]byte, error) {
	target = filepath.Clean(target)
	for _, f := range files {
		if filepath.Clean(strings.ToLower(f.Name)) == strings.ToLower(target) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("epub: file not found: %s", target)
}

func locateOPF(files []*zip.File) (string, error) {
	// Find META-INF/container.xml and read rootfile
	containerData, err := readZipFile(files, "META-INF/container.xml")
	if err != nil {
		return "", fmt.Errorf("epub: container.xml not found: %w", err)
	}
	rootfile := parseRootfile(containerData)
	if rootfile == "" {
		return "", fmt.Errorf("epub: rootfile not found in container.xml")
	}
	return rootfile, nil
}

// parseRootfile reads META-INF/container.xml and returns the first rootfile full-path.
func parseRootfile(data []byte) string {
	type rootfile struct {
		FullPath string `xml:"full-path,attr"`
	}
	type container struct {
		Rootfiles []rootfile `xml:"rootfiles>rootfile"`
	}
	var c container
	if err := xml.Unmarshal(data, &c); err != nil {
		return ""
	}
	if len(c.Rootfiles) == 0 {
		return ""
	}
	return strings.TrimSpace(c.Rootfiles[0].FullPath)
}

// parseOPF parses the package.opf content and returns manifest/spine and basic metadata.
// It is tolerant of namespaces by using a streaming decoder and matching on local names.
func parseOPF(data []byte) (opfPackage, error) {
	type state struct {
		inMetadata bool
		inManifest bool
		inSpine    bool
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	opf := opfPackage{
		Manifest: make(map[string]string),
		Spine:    make([]string, 0, 8),
	}
	st := state{}

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return opfPackage{}, fmt.Errorf("epub: parse opf: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			switch local {
			case "metadata":
				st.inMetadata = true
			case "manifest":
				st.inManifest = true
			case "spine":
				st.inSpine = true
			case "item":
				if st.inManifest {
					var id, href string
					for _, a := range t.Attr {
						switch strings.ToLower(a.Name.Local) {
						case "id":
							id = strings.TrimSpace(a.Value)
						case "href":
							href = strings.TrimSpace(a.Value)
						}
					}
					if id != "" && href != "" {
						opf.Manifest[id] = href
					}
				}
			case "itemref":
				if st.inSpine {
					for _, a := range t.Attr {
						if strings.ToLower(a.Name.Local) == "idref" {
							id := strings.TrimSpace(a.Value)
							if id != "" {
								opf.Spine = append(opf.Spine, id)
							}
						}
					}
				}
			case "title":
				if st.inMetadata {
					var text string
					if err := decoder.DecodeElement(&text, &t); err == nil {
						if opf.Title == "" {
							opf.Title = strings.TrimSpace(text)
						}
					}
				}
			case "creator":
				if st.inMetadata {
					var text string
					if err := decoder.DecodeElement(&text, &t); err == nil {
						if opf.Creator == "" {
							opf.Creator = strings.TrimSpace(text)
						}
					}
				}
			case "date":
				if st.inMetadata {
					var text string
					if err := decoder.DecodeElement(&text, &t); err == nil {
						if opf.Date == "" {
							opf.Date = strings.TrimSpace(text)
						}
					}
				}
			case "subject":
				if st.inMetadata {
					var text string
					if err := decoder.DecodeElement(&text, &t); err == nil {
						txt := strings.TrimSpace(text)
						if txt != "" {
							if opf.Subject == "" {
								opf.Subject = txt
							} else {
								opf.Subject += ", " + txt
							}
						}
					}
				}
			}

		case xml.EndElement:
			local := strings.ToLower(t.Name.Local)
			switch local {
			case "metadata":
				st.inMetadata = false
			case "manifest":
				st.inManifest = false
			case "spine":
				st.inSpine = false
			}
		}
	}

	return opf, nil
}

// openEPUBPackage opens the zip and returns the reader plus parsed OPF.
func openEPUBPackage(path string) (*zip.ReadCloser, opfPackage, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, opfPackage{}, fmt.Errorf("epub: %w", err)
	}
	rootfile, err := locateOPF(r.File)
	if err != nil {
		r.Close()
		return nil, opfPackage{}, err
	}
	opfContent, err := readZipFile(r.File, rootfile)
	if err != nil {
		r.Close()
		return nil, opfPackage{}, err
	}
	opf, err := parseOPF(opfContent)
	if err != nil {
		r.Close()
		return nil, opfPackage{}, err
	}
	opf.rootfilePath = rootfile
	return r, opf, nil
}

// stripHTMLToText collects text nodes from HTML, joining by spaces/newlines.
func stripHTMLToText(data []byte) string {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	var buf strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteString(" ")
				}
				buf.WriteString(text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return buf.String()
}


