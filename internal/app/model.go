package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"gorae/internal/config"
	"gorae/internal/meta"
)

const statusMessageTTL = 4 * time.Second

type sortMode int

const (
	sortByName sortMode = iota
	sortByTitle
	sortByYear
)

type uiState int

const (
	stateNormal uiState = iota
	stateNewDir
	stateConfirmDelete
	stateRename
	stateMetaPreview
	stateEditMeta
	stateCommand
)

type Model struct {
	cfg     *config.Config
	root    string
	cwd     string
	entries []fs.DirEntry
	cursor  int
	err     error

	selected              map[string]bool
	cut                   []string
	status                string
	statusAt              time.Time
	sticky                bool
	commandOutput         []string
	entryTitles           map[string]string
	sortMode              sortMode
	awaitingSort          bool
	recentlyAddedDir      string
	recentlyAddedMaxAge   time.Duration
	recentlyAddedSyncInt  time.Duration
	lastRecentlyAddedSync time.Time

	recentlyOpenedDir   string
	recentlyOpenedLimit int

	viewportStart  int
	viewportHeight int
	width          int

	state        uiState
	input        textinput.Model
	confirmItems []string

	renameTarget string

	meta            *meta.Store   // <── sqlite store
	metaEditingPath string        // path of file being edited
	metaFieldIndex  int           // 0:title,1:author,2:venue,3:year,...
	metaDraft       meta.Metadata // draft being edited

	previewText []string
	previewPath string

	currentMeta     *meta.Metadata
	currentMetaPath string
	currentNote     string
	notesDir        string
	metaPopupOffset int
}

var metaFieldLabels = []string{
	"Title",
	"Author",
	"Journal/Conference",
	"Year",
	"Tag",
	"Abstract",
}

func metaFieldLabel(index int) string {
	if index >= 0 && index < len(metaFieldLabels) {
		return metaFieldLabels[index]
	}
	return ""
}

func metaFieldCount() int {
	return len(metaFieldLabels)
}

func metadataFieldValue(data meta.Metadata, index int) string {
	switch index {
	case 0:
		return data.Title
	case 1:
		return data.Author
	case 2:
		return data.Venue
	case 3:
		return data.Year
	case 4:
		return data.Tag
	case 5:
		return data.Abstract
	default:
		return ""
	}
}

func setMetadataFieldValue(data *meta.Metadata, index int, value string) {
	switch index {
	case 0:
		data.Title = value
	case 1:
		data.Author = value
	case 2:
		data.Venue = value
	case 3:
		data.Year = value
	case 4:
		data.Tag = value
	case 5:
		data.Abstract = value
	}
}

func (m *Model) loadMetaFieldIntoInput() {
	value := metadataFieldValue(m.metaDraft, m.metaFieldIndex)
	m.input.SetValue(value)
	m.input.CursorEnd()
}

func canonicalPath(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != "" {
		path = resolved
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path
}

const (
	panelSeparatorWidth = 6
	minLeftPanelWidth   = 12
	minMiddlePanelWidth = 25
	minRightPanelWidth  = 25
)

func (m Model) panelWidths() (int, int, int) {
	if m.width <= 0 {
		return minLeftPanelWidth, minMiddlePanelWidth, minRightPanelWidth
	}

	leftPct := 0.22
	rightPct := 0.33
	if m.state == stateMetaPreview || m.state == stateEditMeta {
		leftPct = 0.18
		rightPct = 0.28
	}

	left := int(float64(m.width) * leftPct)
	right := int(float64(m.width) * rightPct)

	if left < minLeftPanelWidth {
		left = minLeftPanelWidth
	}
	if right < minRightPanelWidth {
		right = minRightPanelWidth
	}

	middle := m.width - panelSeparatorWidth - left - right
	if middle < minMiddlePanelWidth {
		middle = minMiddlePanelWidth
	}

	total := left + right + middle + panelSeparatorWidth
	if total > m.width {
		overflow := total - m.width

		reduceRight := overflow / 2
		maxReduceRight := right - minRightPanelWidth
		if maxReduceRight < 0 {
			maxReduceRight = 0
		}
		if reduceRight > maxReduceRight {
			reduceRight = maxReduceRight
		}
		right -= reduceRight
		overflow -= reduceRight

		reduceLeft := overflow
		maxReduceLeft := left - minLeftPanelWidth
		if maxReduceLeft < 0 {
			maxReduceLeft = 0
		}
		if reduceLeft > maxReduceLeft {
			reduceLeft = maxReduceLeft
		}
		left -= reduceLeft
		overflow -= reduceLeft

		if overflow > 0 {
			middle -= overflow
			if middle < minMiddlePanelWidth {
				middle = minMiddlePanelWidth
			}
		}
	}

	return left, middle, right
}

func (m *Model) noteFilePath(path string) (string, error) {
	dir := strings.TrimSpace(m.notesDir)
	if dir == "" {
		return "", fmt.Errorf("notes directory not configured")
	}
	target := canonicalPath(path)
	if target == "" {
		return "", fmt.Errorf("invalid note target")
	}
	sum := sha1.Sum([]byte(target))
	name := hex.EncodeToString(sum[:]) + ".md"
	return filepath.Join(dir, name), nil
}

func (m *Model) loadNoteFor(path string) {
	if path == "" {
		m.currentNote = ""
		return
	}
	filePath, err := m.noteFilePath(path)
	if err != nil {
		m.currentNote = ""
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.currentNote = ""
			return
		}
		m.currentNote = ""
		return
	}
	m.currentNote = string(data)
}

func (m *Model) refreshCurrentNote() {
	if m.currentMetaPath == "" {
		m.currentNote = ""
		return
	}
	m.loadNoteFor(m.currentMetaPath)
}

func (m *Model) updateCurrentMetadata(path string) {
	if path == "" || m.meta == nil {
		m.currentMeta = nil
		m.currentMetaPath = ""
		m.currentNote = ""
		return
	}
	canonical := canonicalPath(path)
	info, err := os.Stat(canonical)
	if err != nil || info.IsDir() {
		m.currentMeta = nil
		m.currentMetaPath = ""
		m.currentNote = ""
		return
	}
	if m.currentMetaPath == canonical {
		return
	}
	ctx := context.Background()
	md, err := m.meta.Get(ctx, canonical)
	if err != nil {
		m.currentMeta = nil
		m.currentMetaPath = ""
		m.currentNote = ""
		m.setStatus("Failed to load metadata: " + err.Error())
		return
	}
	m.currentMetaPath = canonical
	m.currentMeta = md
	m.loadNoteFor(canonical)
}

func NewModel(cfg *config.Config, store *meta.Store) Model {
	root := cfg.WatchDir
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 200
	ti.Cursor.Style = ti.Cursor.Style.Bold(true)
	ti.Focus()

	m := Model{
		cfg:                  cfg,
		root:                 root,
		cwd:                  root,
		selected:             make(map[string]bool),
		input:                ti,
		viewportHeight:       20,
		meta:                 store,
		sortMode:             sortByName,
		entryTitles:          make(map[string]string),
		recentlyAddedDir:     strings.TrimSpace(cfg.RecentlyAddedDir),
		recentlyAddedMaxAge:  time.Duration(cfg.RecentlyAddedDays) * 24 * time.Hour,
		recentlyAddedSyncInt: defaultRecentlyAddedSyncInterval,
		recentlyOpenedDir:    strings.TrimSpace(cfg.RecentlyOpenedDir),
		recentlyOpenedLimit:  cfg.RecentlyOpenedLimit,
		notesDir:             strings.TrimSpace(cfg.NotesDir),
	}
	if m.recentlyAddedSyncInt <= 0 {
		m.recentlyAddedSyncInt = defaultRecentlyAddedSyncInterval
	}
	if m.recentlyAddedDir != "" && !filepath.IsAbs(m.recentlyAddedDir) {
		m.recentlyAddedDir = filepath.Join(root, m.recentlyAddedDir)
	}
	if m.recentlyOpenedDir != "" && !filepath.IsAbs(m.recentlyOpenedDir) {
		m.recentlyOpenedDir = filepath.Join(root, m.recentlyOpenedDir)
	}
	if err := m.maybeSyncRecentlyAddedDir(true); err != nil {
		m.setStatus("Recently added sync failed: " + err.Error())
	}
	if m.notesDir == "" && strings.TrimSpace(cfg.MetaDir) != "" {
		m.notesDir = filepath.Join(cfg.MetaDir, "notes")
	}
	if m.notesDir != "" && !filepath.IsAbs(m.notesDir) {
		base := strings.TrimSpace(cfg.MetaDir)
		if base != "" {
			m.notesDir = filepath.Join(base, m.notesDir)
		} else if abs, err := filepath.Abs(m.notesDir); err == nil {
			m.notesDir = abs
		}
	}
	m.loadEntries()
	m.updateTextPreview()
	return m
}

// func (m Model) Init() tea.Cmd {
// 	return nil
// }

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) setStatus(msg string) {
	m.status = msg
	m.statusAt = time.Now()
	m.sticky = false
}

func (m *Model) setPersistentStatus(msg string) {
	m.status = msg
	m.statusAt = time.Now()
	m.sticky = true
}

func sortModeLabel(mode sortMode) string {
	switch mode {
	case sortByTitle:
		return "title"
	case sortByYear:
		return "year"
	default:
		return "name"
	}
}

func (m *Model) applySortMode(mode sortMode) {
	if m.sortMode == mode {
		m.setStatus(fmt.Sprintf("Already sorting by %s", sortModeLabel(mode)))
		return
	}
	m.sortMode = mode
	m.resortAndPreserveSelection()
	m.setStatus(fmt.Sprintf("Sorting by %s", sortModeLabel(mode)))
}

func (m *Model) currentEntryPath() string {
	if len(m.entries) == 0 || m.cursor < 0 || m.cursor >= len(m.entries) {
		return ""
	}
	return filepath.Join(m.cwd, m.entries[m.cursor].Name())
}

func (m *Model) findEntryIndex(path string) int {
	if path == "" {
		return -1
	}
	for i, e := range m.entries {
		full := filepath.Join(m.cwd, e.Name())
		if full == path {
			return i
		}
	}
	return -1
}

func (m *Model) syncCurrentEntryState() {
	if len(m.entries) == 0 {
		m.updateTextPreview()
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	m.updateTextPreview()
}

func (m *Model) resortAndPreserveSelection() {
	current := m.currentEntryPath()
	m.resortEntries()
	if current != "" {
		if idx := m.findEntryIndex(current); idx >= 0 {
			m.cursor = idx
		}
	}
	m.ensureCursorVisible()
	m.syncCurrentEntryState()
}

func (m *Model) clearStatus() {
	m.status = ""
	m.statusAt = time.Time{}
	m.sticky = false
}

func (m Model) statusMessage(now time.Time) string {
	if m.status == "" {
		return ""
	}
	if m.sticky || m.statusAt.IsZero() {
		return m.status
	}
	if now.Sub(m.statusAt) > statusMessageTTL {
		return ""
	}
	return m.status
}

func (m *Model) setCommandOutput(lines []string) {
	m.commandOutput = append([]string{}, lines...)
}

func (m *Model) clearCommandOutput() {
	m.commandOutput = nil
}

func (m *Model) ensureCursorVisible() {
	if m.cursor < m.viewportStart {
		m.viewportStart = m.cursor
	}
	if m.cursor >= m.viewportStart+m.viewportHeight {
		m.viewportStart = m.cursor - m.viewportHeight + 1
	}
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}
}

func (m *Model) updateTextPreview() {
	m.previewText = nil

	if len(m.entries) == 0 {
		m.previewPath = ""
		m.updateCurrentMetadata("")
		return
	}

	e := m.entries[m.cursor]
	full := filepath.Join(m.cwd, e.Name())
	canonical := canonicalPath(full)
	info, err := e.Info()
	isDir := e.IsDir()
	if err == nil {
		isDir = info.IsDir()
	}
	m.updateCurrentMetadata(canonical)

	// Directories: show summary and contents
	if isDir {
		m.previewPath = full
		m.previewText = m.directoryPreviewContents(full)
		return
	}

	if m.currentMeta != nil && m.currentMetaPath == canonical {
		m.previewPath = full
		return
	}

	// Non-PDFs: no text preview
	name := e.Name()
	if err == nil {
		name = info.Name()
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		m.previewPath = full
		m.previewText = []string{
			"No preview (not a PDF)",
			"",
			name,
		}
		return
	}

	// If we already have preview for this file, keep it
	if m.previewPath == full && len(m.previewText) > 0 {
		return
	}

	m.previewPath = full

	// approximate how many lines we can show
	maxLines := m.viewportHeight - 2
	if maxLines < 5 {
		maxLines = 5
	}

	lines, err := extractFirstPageText(full, maxLines)
	if err != nil {
		m.previewText = []string{
			"Preview error:",
			"  " + err.Error(),
		}
		return
	}

	m.previewText = lines
}

func (m *Model) directoryPreviewContents(dir string) []string {
	base := filepath.Base(dir)
	lines := []string{fmt.Sprintf("%s/", base)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		lines = append(lines, "  (error: "+err.Error()+")")
		return lines
	}

	filtered := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		di, dj := filtered[i].IsDir(), filtered[j].IsDir()
		if di != dj {
			return di && !dj
		}
		return strings.ToLower(filtered[i].Name()) < strings.ToLower(filtered[j].Name())
	})

	if len(filtered) == 0 {
		lines = append(lines, "(empty)")
		return lines
	}

	maxLines := m.viewportHeight - 6
	if maxLines < 5 {
		maxLines = 5
	}
	if maxLines > len(filtered) {
		maxLines = len(filtered)
	}
	for i := 0; i < maxLines; i++ {
		name := filtered[i].Name()
		if filtered[i].IsDir() {
			name += "/"
		}
		lines = append(lines, "  "+name)
	}
	if maxLines < len(filtered) {
		lines = append(lines, fmt.Sprintf("  ... %d more", len(filtered)-maxLines))
	}
	return lines
}
