package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorae/internal/meta"
)

// pad or truncate a string to exactly width columns.
func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

// fit or pad lines to a given height.
func fitLines(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// Left panel: simple "tree" panel from root to cwd.
func (m Model) renderTreePanel(width, height int) []string {
	lines := []string{"[Parent]"}

	parent := filepath.Dir(m.cwd)

	// No valid parent under root → just show a note.
	if parent == m.cwd || !strings.HasPrefix(parent, m.root) {
		lines = append(lines, "(no parent under root)")
		lines = trimLinesToWidth(lines, width)
		return fitLines(lines, height)
	}

	ents, err := os.ReadDir(parent)
	if err != nil {
		lines = append(lines, "(error reading parent)")
		lines = trimLinesToWidth(lines, width)
		return fitLines(lines, height)
	}

	// hide dotfiles
	filtered := make([]os.DirEntry, 0, len(ents))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		filtered = append(filtered, e)
	}

	// Optionally: sort dirs first, then alphabetically
	sort.SliceStable(filtered, func(i, j int) bool {
		di, dj := filtered[i].IsDir(), filtered[j].IsDir()
		if di != dj {
			return di && !dj
		}
		return strings.ToLower(filtered[i].Name()) <
			strings.ToLower(filtered[j].Name())
	})

	lines = append(lines, parent)

	for _, e := range filtered {
		full := filepath.Join(parent, e.Name())

		// mark the current directory
		marker := "  "
		if full == m.cwd {
			marker = "➜ "
		}

		name := e.Name()
		if e.IsDir() {
			name += "/"
		}

		lines = append(lines, marker+name)
	}

	return fitLines(lines, height)
}

// Middle panel: file list (what your old View used to show).
func (m Model) renderListPanel(width, height int) []string {
	var lines []string

	if len(m.entries) == 0 {
		lines = append(lines, "(empty)")
		lines = trimLinesToWidth(lines, width)
		return fitLines(lines, height)
	}

	end := m.viewportStart + height
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.viewportStart; i < end; i++ {
		e := m.entries[i]

		cursor := "  "
		if i == m.cursor {
			cursor = "➜ "
		}

		full := filepath.Join(m.cwd, e.Name())
		sel := "[ ] "
		if m.selected[full] {
			sel = "[x] "
		}

		var line string

		display := m.entryDisplayName(full, e)
		line = fmt.Sprintf("%s%s %s", cursor, sel, display)

		lines = append(lines, line)
	}

	return fitLines(lines, height)
}

func (m Model) renderPreviewPanel(width, height int) []string {
	if height <= 0 {
		return nil
	}

	showMetadataOnly := false
	if len(m.entries) > 0 {
		entry := m.entries[m.cursor]
		if !entry.IsDir() {
			full := filepath.Join(m.cwd, entry.Name())
			canonical := canonicalPath(full)
			showMetadataOnly = canonical == m.currentMetaPath && m.currentMeta != nil
		}
	}

	metaSection := m.metadataPanelLines(width)
	if showMetadataOnly && len(metaSection) > 0 {
		return fitLines(metaSection, height)
	}

	previewSection := m.previewPanelLines(width)
	if len(metaSection) == 0 {
		if len(previewSection) > height {
			previewSection = previewSection[:height]
		}
		return fitLines(previewSection, height)
	}

	reservedMeta := height / 3
	if reservedMeta < 6 {
		if height >= 6 {
			reservedMeta = 6
		} else if height > 2 {
			reservedMeta = height / 2
		} else {
			reservedMeta = height
		}
	}
	if reservedMeta > height {
		reservedMeta = height
	}

	previewLimit := height - reservedMeta
	if previewLimit < 0 {
		previewLimit = 0
	}
	if previewLimit > len(previewSection) {
		previewLimit = len(previewSection)
	}

	lines := make([]string, 0, height)
	appendSection := func(section []string) {
		for _, line := range section {
			if len(lines) >= height {
				return
			}
			lines = append(lines, line)
		}
	}

	appendSection(previewSection[:previewLimit])
	if previewLimit > 0 && len(lines) < height {
		lines = append(lines, dividerLine(width))
	}

	remaining := height - len(lines)
	if remaining < 0 {
		remaining = 0
	}
	metaCount := len(metaSection)
	if metaCount > remaining {
		metaCount = remaining
	}
	appendSection(metaSection[:metaCount])

	return fitLines(lines, height)
}

// // wrapLinesToWidth wraps each line so that no visual line exceeds `width` runes.
// func wrapLinesToWidth(lines []string, width int) []string {
// 	if width <= 0 {
// 		return nil
// 	}

// 	var out []string
// 	for _, l := range lines {
// 		r := []rune(l)
// 		for len(r) > width {
// 			out = append(out, string(r[:width]))
// 			r = r[width:]
// 		}
// 		out = append(out, string(r))
// 	}
// 	return out
// }

// trim each line so it never exceeds the given width (in runes).
func trimLinesToWidth(lines []string, width int) []string {
	if width <= 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		r := []rune(l)
		if len(r) > width {
			out[i] = string(r[:width])
		} else {
			out[i] = l
		}
	}
	return out
}

func wrapTextToWidth(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := ""
	appendCurrent := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
	}
	for _, word := range words {
		wordRunes := []rune(word)
		if runeLen(word) > width {
			appendCurrent()
			for len(wordRunes) > width {
				lines = append(lines, string(wordRunes[:width]))
				wordRunes = wordRunes[width:]
			}
			current = string(wordRunes)
			continue
		}
		if current == "" {
			current = word
			continue
		}
		candidate := current + " " + word
		if runeLen(candidate) > width {
			lines = append(lines, current)
			current = word
		} else {
			current = candidate
		}
	}
	appendCurrent()
	return lines
}

func (m Model) renderMetaPopupLines(width int) []string {
	label := metaFieldLabel(m.metaFieldIndex)
	if label == "" {
		label = "Field"
	}
	fileName := filepath.Base(m.metaEditingPath)
	if fileName == "" || fileName == "." {
		fileName = m.metaEditingPath
	}

	popupLines := []string{
		fmt.Sprintf("File : %s", fileName),
		"",
		"Fields:",
	}

	for i := 0; i < metaFieldCount(); i++ {
		fieldLabel := metaFieldLabel(i)
		value := strings.TrimSpace(metadataFieldValue(m.metaDraft, i))
		if value == "" {
			value = "(empty)"
		}
		prefix := "  "
		if m.metaFieldIndex == i {
			prefix = "➤ "
		}
		popupLines = append(popupLines, fmt.Sprintf("%s%s: %s", prefix, fieldLabel, value))
	}

	if m.state == stateEditMeta {
		popupLines = append(popupLines,
			"",
			fmt.Sprintf("Edit %s:", label),
			m.input.View(),
			"",
			"Tab       → next field",
			"Shift+Tab → previous field",
			"Enter     → next/save",
			"Esc       → cancel",
		)
	} else {
		popupLines = append(popupLines,
			"",
			"Press 'e' again to edit metadata.",
			"Press Esc to cancel.",
		)
	}

	box := renderPopupBox("Metadata Editor", popupLines, width)
	box = strings.TrimRight(box, "\n")
	if box == "" {
		return nil
	}
	return strings.Split(box, "\n")
}

func renderPopupBox(title string, lines []string, totalWidth int) string {
	if totalWidth <= 0 {
		totalWidth = 80
	}

	maxLen := runeLen(title)
	for _, line := range lines {
		if l := runeLen(line); l > maxLen {
			maxLen = l
		}
	}

	boxWidth := maxLen
	if boxWidth < 30 {
		boxWidth = 30
	}
	if limit := totalWidth - 4; limit > 10 && boxWidth > limit {
		boxWidth = limit
	}
	if boxWidth < 10 {
		boxWidth = 10
	}

	boxLineWidth := boxWidth + 4
	indent := 0
	if totalWidth > boxLineWidth {
		indent = (totalWidth - boxLineWidth) / 2
	}
	pad := strings.Repeat(" ", indent)

	horizontal := "+" + strings.Repeat("-", boxWidth+2) + "+\n"

	var b strings.Builder
	b.WriteString(pad)
	b.WriteString(horizontal)
	b.WriteString(pad)
	b.WriteString(fmt.Sprintf("| %s |\n", padRight(title, boxWidth)))
	b.WriteString(pad)
	b.WriteString("| " + strings.Repeat("-", boxWidth) + " |\n")
	for _, line := range lines {
		b.WriteString(pad)
		b.WriteString(fmt.Sprintf("| %s |\n", padRight(line, boxWidth)))
	}
	b.WriteString(pad)
	b.WriteString(horizontal)

	return b.String()
}

func runeLen(s string) int {
	return len([]rune(s))
}

func (m Model) View() string {
	var b strings.Builder
	var overlayLines []string

	// Header (full width)
	fmt.Fprintf(&b, "Dir : %s\n\n", m.cwd)

	// If we don't know width yet (no WindowSizeMsg yet), fall back to single-panel list.
	if m.width <= 0 {
		for _, line := range m.renderListPanel(80, m.viewportHeight) {
			b.WriteString(line + "\n")
		}
	} else {
		// --- compute panel widths ---
		// // Left: 1/4, Right: 1/3, Middle: remaining.
		// leftWidth := m.width / 5
		// rightWidth := m.width / 3 + 10
		// middleWidth := m.width - leftWidth - rightWidth - 2  // 2 for "│" separators

		separatorWidth := 6                        // " │ " + " │ "
		leftWidth := int(float64(m.width) * 0.22)  // 22%
		rightWidth := int(float64(m.width) * 0.33) // 33%
		middleWidth := m.width - leftWidth - rightWidth - separatorWidth

		if leftWidth < 12 {
			leftWidth = 12
		}
		if middleWidth < 25 {
			middleWidth = 25
		}
		if rightWidth < 25 {
			rightWidth = 25
		}

		height := m.viewportHeight

		treeLines := m.renderTreePanel(leftWidth, height)
		listLines := m.renderListPanel(middleWidth, height)
		prevLines := m.renderPreviewPanel(rightWidth, height)

		if m.state == stateEditMeta || m.state == stateMetaPreview {
			overlayLines = m.renderMetaPopupLines(middleWidth)
			if len(overlayLines) > 0 {
				overlayLines = trimLinesToWidth(overlayLines, middleWidth)
			}
		}

		for i := 0; i < height; i++ {
			tl := ""
			if i < len(treeLines) {
				tl = treeLines[i]
			}
			ll := ""
			if len(overlayLines) > 0 && i < len(overlayLines) {
				ll = overlayLines[i]
			} else if i < len(listLines) {
				ll = listLines[i]
			}
			pl := ""
			if i < len(prevLines) {
				pl = prevLines[i]
			}

			line := padRight(tl, leftWidth) +
				"" +
				padRight(ll, middleWidth) +
				"" +
				padRight(pl, rightWidth)

			b.WriteString(line + "\n")
		}
	}

	// Footer
	if m.state == stateNewDir {
		fmt.Fprintf(&b, "\nCreate directory: %s\n", m.input.View())
	} else if m.state == stateRename {
		fmt.Fprintf(&b, "\nRename: %s\n", m.input.View())
	}
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	if m.state == stateCommand {
		fmt.Fprintf(&b, "Command: %s\n", m.input.View())
	}
	if len(m.commandOutput) > 0 {
		lines := m.commandOutput
		if m.width > 0 {
			lines = trimLinesToWidth(lines, m.width)
		}
		for _, line := range lines {
			b.WriteString(line + "\n")
		}
	}

	return b.String()
}

func (m Model) metadataPreviewLines(width int) []string {
	if m.meta == nil || m.currentMetaPath == "" {
		return nil
	}
	var md meta.Metadata
	if m.currentMeta != nil {
		md = *m.currentMeta
	}
	md.Path = m.currentMetaPath
	lines := make([]string, 0, metaFieldCount()+1)
	lines = append(lines, "Metadata:")
	for i := 0; i < metaFieldCount(); i++ {
		val := strings.TrimSpace(metadataFieldValue(md, i))
		if val == "" {
			val = "(empty)"
		}
		label := metaFieldLabel(i)
		if strings.EqualFold(label, "Abstract") {
			lines = append(lines, label+":")
			offsetWidth := width - 2
			if offsetWidth < 10 {
				offsetWidth = width
			}
			wrapped := wrapTextToWidth(val, offsetWidth)
			for _, w := range wrapped {
				lines = append(lines, "  "+w)
			}
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, val))
	}
	return lines
}

func (m Model) renderStatusBar() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	dirInfo := fmt.Sprintf("Dir: %s", m.cwd)
	itemInfo := m.selectionSummary()
	status := m.statusMessage(time.Now())
	if status == "" {
		status = "Ready"
	}

	line := fmt.Sprintf(" %s │ %s │ %s ", dirInfo, itemInfo, status)
	r := []rune(line)
	if len(r) > width {
		line = string(r[:width])
	} else if len(r) < width {
		line += strings.Repeat(" ", width-len(r))
	}
	return line
}

func (m Model) selectionSummary() string {
	selectedCount := len(m.selected)
	if len(m.entries) == 0 {
		if selectedCount > 0 {
			return fmt.Sprintf("Selected: %d", selectedCount)
		}
		return "No items"
	}

	entry := m.entries[m.cursor]
	name := entry.Name()
	if entry.IsDir() {
		name += "/"
	}

	info := "Item: " + name
	if selectedCount > 0 {
		info += fmt.Sprintf("  Sel:%d", selectedCount)
	}
	return info
}

func (m Model) entryDisplayName(full string, entry fs.DirEntry) string {
	if title, ok := m.entryTitles[full]; ok && title != "" {
		return title
	}
	if entry.IsDir() {
		return entry.Name() + "/"
	}
	name := entry.Name()
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

func (m Model) metadataPanelLines(width int) []string {
	metaLines := m.metadataPreviewLines(width)
	if len(metaLines) == 0 {
		return nil
	}
	return trimLinesToWidth(metaLines, width)
}

func (m Model) previewPanelLines(width int) []string {
	lines := []string{}

	if len(m.entries) == 0 {
		return trimLinesToWidth([]string{"No selection"}, width)
	}

	if len(m.previewText) > 0 {
		preview := make([]string, len(m.previewText))
		copy(preview, m.previewText)
		return trimLinesToWidth(preview, width)
	}

	e := m.entries[m.cursor]
	full := filepath.Join(m.cwd, e.Name())
	display := m.entryDisplayName(full, e)

	if e.IsDir() {
		lines = append(lines, display)
	} else {
		lines = append(lines,
			"File:",
			"  "+display,
			"",
			"Path:",
			"  "+full,
		)
	}
	return trimLinesToWidth(lines, width)
}

func dividerLine(width int) string {
	if width <= 0 {
		width = 40
	}
	return strings.Repeat("─", width)
}
