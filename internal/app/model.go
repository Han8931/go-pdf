package app

import (
	"io/fs"

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type uiState int

const (
	stateNormal uiState = iota
	stateNewDir
	stateConfirmDelete
)

type Model struct {
	root    string
	cwd     string
	entries []fs.DirEntry
	cursor  int
	err     error

	selected map[string]bool
	cut      []string
	status   string

	viewportStart  int
	viewportHeight int

	state        uiState
	input        textinput.Model // textinput for new directory
	confirmItems []string        // paths pending delete
}

func NewModel(root string) Model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 200
	ti.Cursor.Style = ti.Cursor.Style.Bold(true) // new API: Cursor.Style
	ti.Focus()

	m := Model{
		root:           root,
		cwd:            root,
		selected:       make(map[string]bool),
		input:          ti,
		viewportHeight: 20,
	}
	m.loadEntries()
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	// Alt-screen is handled by tea.WithAltScreen() in main, so nothing to do here.
	return nil
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
