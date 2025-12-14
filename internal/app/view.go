package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

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

type panelLineKind int

const (
	panelLineBody panelLineKind = iota
	panelLineInfo
	panelLineActive
	panelLineSelected
	panelLineCursor
	panelLineCursorSelected
)

type panelLine struct {
	text string
	kind panelLineKind
}

// Left panel: simple "tree" panel from root to cwd.
func (m Model) renderTreePanel(width, height int) []string {
	lines := []panelLine{
		{text: fmt.Sprintf("Current: %s", filepath.Base(m.cwd)), kind: panelLineInfo},
	}

	parent := filepath.Dir(m.cwd)
	if parent == m.cwd || !strings.HasPrefix(parent, m.root) {
		lines = append(lines, panelLine{text: "(root directory)", kind: panelLineInfo})
		return m.renderPanelBlock("Tree", lines, width, height, m.styles.Tree)
	}

	ents, err := os.ReadDir(parent)
	if err != nil {
		lines = append(lines, panelLine{text: "(error reading parent)", kind: panelLineInfo})
		return m.renderPanelBlock("Tree", lines, width, height, m.styles.Tree)
	}

	filtered := make([]os.DirEntry, 0, len(ents))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		filtered = append(filtered, e)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		di, dj := filtered[i].IsDir(), filtered[j].IsDir()
		if di != dj {
			return di && !dj
		}
		return strings.ToLower(filtered[i].Name()) <
			strings.ToLower(filtered[j].Name())
	})

	lines = append(lines, panelLine{text: fmt.Sprintf("Parent: %s", parent), kind: panelLineInfo})
	for _, e := range filtered {
		full := filepath.Join(parent, e.Name())
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		icon := m.entryIcon(e.IsDir())
		text := fmt.Sprintf("%s %s", icon, name)
		kind := panelLineBody
		if full == m.cwd {
			kind = panelLineActive
		}
		lines = append(lines, panelLine{text: text, kind: kind})
	}

	return m.renderPanelBlock("Tree", lines, width, height, m.styles.Tree)
}

// Middle panel: file list (what your old View used to show).
func (m Model) renderListPanel(width, height int) []string {
	var lines []panelLine

	if len(m.entries) == 0 {
		lines = append(lines, panelLine{text: "(empty)", kind: panelLineInfo})
		return m.renderPanelBlock("Files", lines, width, height, m.styles.List)
	}

	bodyRows := height - 3
	if bodyRows < 1 {
		bodyRows = 1
	}
	end := m.viewportStart + bodyRows
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.viewportStart; i < end; i++ {
		e := m.entries[i]
		full := filepath.Join(m.cwd, e.Name())
		display := m.entryDisplayName(full, e)

		kind := panelLineBody
		if i == m.cursor {
			kind = panelLineCursor
		}

		selMarker := " "
		if m.selected[full] {
			selMarker = m.selectionIndicator()
		}

		text := fmt.Sprintf("%s %s", selMarker, display)
		lines = append(lines, panelLine{text: text, kind: kind})
	}

	title := fmt.Sprintf("Files (%d)", len(m.entries))
	return m.renderPanelBlock(title, lines, width, height, m.styles.List)
}

func (m Model) renderPreviewPanel(width, height int) []string {
	if height <= 0 {
		return nil
	}
	innerWidth := width - 2
	if innerWidth <= 0 {
		innerWidth = width
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

	metaSection := panelizeLines(m.metadataPanelLines(innerWidth))
	if showMetadataOnly && len(metaSection) > 0 {
		return m.renderPanelBlock("Details", metaSection, width, height, m.styles.Preview)
	}

	previewSection := panelizeLines(m.previewPanelLines(innerWidth))
	if len(metaSection) == 0 {
		return m.renderPanelBlock("Details", previewSection, width, height, m.styles.Preview)
	}

	reservedMeta := height / 2
	if reservedMeta < 8 {
		if height >= 8 {
			reservedMeta = 8
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

	lines := make([]panelLine, 0, height)
	lines = append(lines, previewSection[:previewLimit]...)
	if previewLimit > 0 && len(lines) < height {
		lines = append(lines, panelLine{
			text: dividerLine(innerWidth),
			kind: panelLineInfo,
		})
	}

	remaining := height - len(lines)
	if remaining < 0 {
		remaining = 0
	}
	metaCount := len(metaSection)
	if metaCount > remaining {
		metaCount = remaining
	}
	lines = append(lines, metaSection[:metaCount]...)

	return m.renderPanelBlock("Details", lines, width, height, m.styles.Preview)
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
		out[i] = trimStringToWidth(l, width)
	}
	return out
}

func trimLine(s string, width int) string {
	return trimStringToWidth(s, width)
}

func trimStringToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	current := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if current+w > width {
			break
		}
		b.WriteRune(r)
		current += w
	}
	return b.String()
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

func isParagraphMetaField(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "abstract", "note":
		return true
	default:
		return false
	}
}

func boolLabel(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func (m Model) renderMetaPopupLines(width int) []string {
	lines := m.metaPopupContentLines(width)
	if len(lines) == 0 {
		return nil
	}
	height := m.viewportHeight
	if height <= 0 {
		height = len(lines)
	}
	if height <= 0 {
		return nil
	}
	maxOffset := len(lines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.metaPopupOffset
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

func (m Model) metaPopupContentLines(width int) []string {
	if width <= 0 {
		width = 40
	}
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

	wrapWidth := width - 6
	if wrapWidth < 10 {
		wrapWidth = width
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

		if isParagraphMetaField(fieldLabel) {
			popupLines = append(popupLines, fmt.Sprintf("%s%s:", prefix, fieldLabel))
			wrapped := wrapTextToWidth(value, wrapWidth)
			for _, line := range wrapped {
				popupLines = append(popupLines, "    "+line)
			}
			continue
		}

		popupLines = append(popupLines, fmt.Sprintf("%s%s: %s", prefix, fieldLabel, value))
	}

	popupLines = append(popupLines, "", "Note preview:")
	note := strings.TrimSpace(m.currentNote)
	if note == "" {
		popupLines = append(popupLines, "    (none - press 'n' to edit)")
	} else {
		for _, line := range wrapTextToWidth(note, wrapWidth) {
			popupLines = append(popupLines, "    "+line)
		}
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
			"Esc or q  → cancel",
		)
	} else {
		popupLines = append(popupLines,
			"",
			"Use ↑/↓ or PgUp/PgDn to scroll fields.",
			"Press 'e' to edit fields here, 'v' to edit fields in your editor.",
			"Press 'n' to edit the note in your editor.",
			"Press 'Esc' or 'q' to cancel.",
		)
	}

	box := renderPopupBox("Metadata Editor", popupLines, width)
	box = strings.TrimRight(box, "\n")
	if box == "" {
		return nil
	}
	lines := strings.Split(box, "\n")
	for i, line := range lines {
		lines[i] = m.styles.MetaOverlay.Render(line)
	}
	return lines
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
	if m.state == stateSearchResults {
		return m.renderSearchResultsView()
	}
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

		leftWidth, middleWidth, rightWidth := m.panelWidths()

		height := m.viewportHeight
		if height < 3 {
			height = 3
		}

		treeLines := m.renderTreePanel(leftWidth, height)
		listLines := m.renderListPanel(middleWidth, height)
		prevLines := m.renderPreviewPanel(rightWidth, height)

		if m.state == stateEditMeta || m.state == stateMetaPreview {
			overlayLines = m.renderMetaPopupLines(middleWidth)
			if len(overlayLines) > 0 {
				for i := range overlayLines {
					overlayLines[i] = padStyledLine(overlayLines[i], middleWidth)
				}
			}
		}

		gapWidth := panelSeparatorWidth / 2
		if gapWidth < 1 {
			gapWidth = 1
		}
		gap := strings.Repeat(" ", gapWidth)

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

			line := tl
			if gap != "" {
				line += gap
			}
			line += ll
			if gap != "" {
				line += gap
			}
			line += pl

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
	} else if m.state == stateSearchPrompt {
		fmt.Fprintf(&b, "Search: %s\n", m.input.View())
	} else if m.state == stateArxivPrompt {
		fmt.Fprintf(&b, "arXiv ID: %s\n", m.input.View())
	}
	if len(m.commandOutput) > 0 {
		lines := m.commandOutput
		if m.width > 0 {
			lines = trimLinesToWidth(lines, m.width)
		}
		start := 0
		end := len(lines)
		if m.commandOutputPinned && len(lines) > 0 {
			view := m.commandOutputViewHeight()
			if view > len(lines) {
				view = len(lines)
			}
			maxOffset := len(lines) - view
			offset := m.commandOutputOffset
			if offset < 0 {
				offset = 0
			}
			if maxOffset < 0 {
				maxOffset = 0
			}
			if offset > maxOffset {
				offset = maxOffset
			}
			start = offset
			end = offset + view
			if end > len(lines) {
				end = len(lines)
			}
		}
		for _, line := range lines[start:end] {
			b.WriteString(line + "\n")
		}
		if m.commandOutputPinned && len(lines) > 0 {
			summary := fmt.Sprintf("-- lines %d-%d of %d (j/k scroll, Esc close) --", start+1, end, len(lines))
			if m.width > 0 {
				summary = trimLinesToWidth([]string{summary}, m.width)[0]
			}
			b.WriteString(summary + "\n")
		}
	}

	return b.String()
}

func (m Model) renderSearchResultsView() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.windowHeight
	if height <= 0 {
		height = m.viewportHeight + 5
	}
	if height <= 0 {
		height = 24
	}
	listHeight, detailHeight := m.searchResultsHeights()

	var b strings.Builder
	modeName := m.lastSearchMode.displayName()
	if modeName == "" {
		modeName = "Content"
	}
	fmt.Fprintf(&b, "Search results (%s): %q\n", modeName, strings.TrimSpace(m.lastSearchQuery))
	if summary := strings.TrimSpace(m.searchSummary); summary != "" {
		b.WriteString(trimLine(summary, width) + "\n")
	}
	b.WriteString("Controls: j/k move • PgUp/PgDn page • Enter open • Esc/q close • / search again\n\n")

	if len(m.searchResults) == 0 {
		b.WriteString("(no matches)\n")
	} else {
		start := m.searchResultOffset
		end := start + listHeight
		if end > len(m.searchResults) {
			end = len(m.searchResults)
		}
		for i := start; i < end; i++ {
			match := m.searchResults[i]
			cursor := "  "
			if i == m.searchResultCursor {
				cursor = "➜ "
			}
			countInfo := ""
			if match.Mode == searchModeContent && match.MatchCount > 0 {
				countInfo = fmt.Sprintf(" (%d)", match.MatchCount)
			}
			line := fmt.Sprintf("%s%s%s", cursor, match.Path, countInfo)
			b.WriteString(trimLine(line, width) + "\n")
		}
		b.WriteString(fmt.Sprintf("-- results %d-%d of %d --\n", start+1, end, len(m.searchResults)))
	}

	b.WriteString(dividerLine(width) + "\n")

	detailLines := m.searchResultDetailLines(detailHeight, width)
	for _, line := range detailLines {
		b.WriteString(line + "\n")
	}

	if len(m.searchWarnings) > 0 {
		maxWarn := detailHeight / 2
		if maxWarn < 1 {
			maxWarn = 1
		}
		if maxWarn > len(m.searchWarnings) {
			maxWarn = len(m.searchWarnings)
		}
		b.WriteString("\nWarnings:\n")
		for i := 0; i < maxWarn; i++ {
			b.WriteString(trimLine(m.searchWarnings[i], width) + "\n")
		}
		if len(m.searchWarnings) > maxWarn {
			b.WriteString(fmt.Sprintf("... %d more warning(s)\n", len(m.searchWarnings)-maxWarn))
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
		if isParagraphMetaField(label) {
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
	lines = append(lines, "Status:")
	lines = append(lines, "  Favorite: "+boolLabel(md.Favorite))
	lines = append(lines, "  To-read : "+boolLabel(md.ToRead))
	lines = append(lines, fmt.Sprintf("  Reading : %s %s", m.readingStateIcon(md.ReadingState), readingStateLabel(md.ReadingState)))
	lines = append(lines, "")
	noteWidth := width - 2
	if noteWidth < 10 {
		noteWidth = width
	}
	lines = append(lines, "Note:")
	note := strings.TrimSpace(m.currentNote)
	if note == "" {
		lines = append(lines, "  (none - press 'n' to edit in your editor)")
	} else {
		for _, wrapped := range wrapTextToWidth(note, noteWidth) {
			lines = append(lines, "  "+wrapped)
		}
	}
	return lines
}

func (m Model) renderStatusBar() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	dirSeg := m.statusSegment("Dir", m.cwd)
	label, value := m.selectionSummary()
	itemSeg := m.statusSegment(label, value)
	status := m.statusMessage(time.Now())
	if status == "" {
		status = "Ready"
	}
	statusSeg := m.statusSegment("Status", status)
	sep := m.styles.Separator.Render("│")

	segments := []string{dirSeg, sep, itemSeg, sep, statusSeg}
	line := strings.Join(segments, " ")
	line = padStyledLine(line, width)
	return m.styles.StatusBar.Render(line)
}

func (m Model) selectionSummary() (string, string) {
	selectedCount := len(m.selected)
	if len(m.entries) == 0 {
		if selectedCount > 0 {
			return "Selected", fmt.Sprintf("%d", selectedCount)
		}
		return "Items", "0"
	}

	entry := m.entries[m.cursor]
	name := entry.Name()
	if entry.IsDir() {
		name += "/"
	}

	value := name
	if selectedCount > 0 {
		value += fmt.Sprintf("  Sel:%d", selectedCount)
	}
	return "Item", value
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

func (m Model) searchResultDetailLines(limit, width int) []string {
	match := m.currentSearchMatch()
	if match == nil {
		lines := []string{
			"(no selection)",
			"",
			"Press Esc to exit the search view.",
		}
		return trimLinesToWidth(lines, width)
	}
	lines := []string{
		fmt.Sprintf("File: %s", match.Path),
		fmt.Sprintf("Matches: %d", match.MatchCount),
		"",
	}
	if match.Mode == searchModeContent {
		lines = append(lines, "Snippets:")
		lines = append(lines, formatContentSnippets(match.Snippets)...)
	} else {
		lines = append(lines, "Metadata:")
		for _, snippet := range match.Snippets {
			lines = append(lines, "  "+snippet)
		}
	}
	lines = trimLinesToWidth(lines, width)
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}
	return lines
}

func formatContentSnippets(snippets []string) []string {
	if len(snippets) == 0 {
		return []string{"  (no snippet data)"}
	}
	lines := make([]string, 0, len(snippets)*3)
	for i, snippet := range snippets {
		parts := strings.Split(snippet, "\n")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			lines = append(lines, "  "+part)
		}
		if i < len(snippets)-1 {
			lines = append(lines, "")
		}
	}
	if len(lines) == 0 {
		lines = []string{"  (no snippet data)"}
	}
	return lines
}

func panelizeLines(lines []string) []panelLine {
	out := make([]panelLine, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		kind := panelLineBody
		if trimmed == "" {
			kind = panelLineBody
		} else if !strings.HasPrefix(line, "  ") && strings.HasSuffix(trimmed, ":") {
			kind = panelLineInfo
		}
		out = append(out, panelLine{text: line, kind: kind})
	}
	return out
}

func padStyledLine(line string, width int) string {
	w := lipgloss.Width(line)
	if w >= width {
		return lipgloss.PlaceHorizontal(width, lipgloss.Left, line)
	}
	return line + strings.Repeat(" ", width-w)
}

func (m Model) statusSegment(label, value string) string {
	lbl := m.styles.StatusLabel.Render(strings.TrimSpace(label))
	val := m.styles.StatusValue.Render(strings.TrimSpace(value))
	return fmt.Sprintf("%s %s", lbl, val)
}

func (m Model) renderPanelBlock(title string, lines []panelLine, width, height int, styles panelStyles) []string {
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}
	innerWidth := width - 2
	bodyHeight := height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	top := m.styles.Border.Render(m.borderChars.TopLeft + strings.Repeat(m.borderChars.Horizontal, innerWidth) + m.borderChars.TopRight)
	result := []string{top}

	header := panelContent(innerWidth, title)
	headerLine := fallbackStyle(styles.Header, lipgloss.NewStyle()).Render(header)
	result = append(result, m.borderRow(headerLine, width))

	bodyIndex := 0
	for i := 0; i < bodyHeight-1; i++ {
		text := ""
		kind := panelLineBody
		if bodyIndex < len(lines) {
			entry := lines[bodyIndex]
			bodyIndex++
			text = entry.text
			kind = entry.kind
		}
		content := panelContent(innerWidth, text)
		styled := m.styleForPanelLine(styles, kind).Render(content)
		result = append(result, m.borderRow(styled, width))
	}

	bottom := m.styles.Border.Render(m.borderChars.BottomLeft + strings.Repeat(m.borderChars.Horizontal, innerWidth) + m.borderChars.BottomRight)
	result = append(result, bottom)
	return result
}

func panelContent(innerWidth int, text string) string {
	if innerWidth <= 0 {
		return ""
	}
	margin := 1
	if innerWidth <= margin*2 {
		margin = 0
	}
	usable := innerWidth - margin*2
	if usable <= 0 {
		usable = innerWidth
		margin = 0
	}
	trimmed := trimLine(text, usable)
	padded := padStyledLine(trimmed, usable)
	if margin == 0 {
		return padded
	}
	return strings.Repeat(" ", margin) + padded + strings.Repeat(" ", margin)
}

func (m Model) borderRow(content string, width int) string {
	if width <= 2 {
		return content
	}
	left := m.styles.Border.Render(m.borderChars.Vertical)
	right := m.styles.Border.Render(m.borderChars.Vertical)
	return left + content + right
}

func (m Model) styleForPanelLine(styles panelStyles, kind panelLineKind) lipgloss.Style {
	switch kind {
	case panelLineInfo:
		return fallbackStyle(styles.Info, styles.Body)
	case panelLineActive:
		return fallbackStyle(styles.Active, styles.Body)
	case panelLineSelected:
		return fallbackStyle(styles.Selected, styles.Body)
	case panelLineCursor:
		return fallbackStyle(styles.Cursor, fallbackStyle(styles.Selected, styles.Body))
	case panelLineCursorSelected:
		if !isZeroStyle(styles.CursorSelected) {
			return styles.CursorSelected
		}
		if !isZeroStyle(styles.Cursor) {
			return styles.Cursor
		}
		return fallbackStyle(styles.Selected, styles.Body)
	default:
		return fallbackStyle(styles.Body, lipgloss.NewStyle())
	}
}

func fallbackStyle(primary, fallback lipgloss.Style) lipgloss.Style {
	if isZeroStyle(primary) {
		return fallback
	}
	return primary
}

func isZeroStyle(s lipgloss.Style) bool {
	return reflect.ValueOf(s).IsZero()
}
