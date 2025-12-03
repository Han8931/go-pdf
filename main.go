package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type uiState int

const (
	stateNormal uiState = iota
	stateNewDir
	stateConfirmDelete
)

type model struct {
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

func newModel(root string) model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 200
	ti.Cursor.Style = ti.Cursor.Style.Bold(true)
	ti.Focus()

	m := model{
		root:           root,
		cwd:            root,
		selected:       make(map[string]bool),
		input:          ti,
		viewportHeight: 20,
	}
	m.loadEntries()
	return m
}

func (m *model) loadEntries() {
	ents, err := os.ReadDir(m.cwd)
	m.err = err
	if err != nil {
		m.entries = nil
		m.cursor = 0
		return
	}

	// hide dotfiles
	filtered := make([]fs.DirEntry, 0, len(ents))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		filtered = append(filtered, e)
	}

	// sort dirs first then alpha
	sort.SliceStable(filtered, func(i, j int) bool {
		di, dj := filtered[i].IsDir(), filtered[j].IsDir()
		if di != dj {
			return di && !dj
		}
		return strings.ToLower(filtered[i].Name()) <
			strings.ToLower(filtered[j].Name())
	})

	m.entries = filtered

	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m *model) ensureCursorVisible() {
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

func (m *model) moveCursor(delta int) {
    if len(m.entries) == 0 { return }
    m.cursor += delta
    if m.cursor < 0 { m.cursor = 0 }
    if m.cursor > len(m.entries)-1 { m.cursor = len(m.entries)-1 }
    m.ensureCursorVisible()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tea.ClearScreen,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.viewportHeight = msg.Height - 5
		if m.viewportHeight < 1 {
			m.viewportHeight = 1
		}
		m.ensureCursorVisible()
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// ===========================
		//  NEW DIRECTORY MODE
		// ===========================
		if m.state == stateNewDir {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)

			switch key {
			case "enter":
				name := strings.TrimSpace(m.input.Value())
				m.state = stateNormal
				m.input.SetValue("")

				if name == "" {
					m.status = "Directory name cannot be empty"
					return m, cmd
				}
				if strings.HasPrefix(name, ".") {
					m.status = "Dot directories are hidden; choose another name"
					return m, cmd
				}

				dst := filepath.Join(m.cwd, name)
				if _, err := os.Stat(dst); err == nil {
					m.status = "Already exists"
					return m, cmd
				}

				if err := os.MkdirAll(dst, 0o755); err != nil {
					m.status = "Failed: " + err.Error()
					return m, cmd
				}

				m.loadEntries()

				// jump to new folder
				for i, e := range m.entries {
					if e.IsDir() && e.Name() == name {
						m.cursor = i
						break
					}
				}
				m.ensureCursorVisible()
				m.status = "Directory created"
				return m, cmd

			case "esc":
				m.state = stateNormal
				m.status = "Cancelled"
				m.input.SetValue("")
				return m, cmd
			}

			return m, cmd
		}

		// ===========================
		// DELETE CONFIRMATION MODE
		// ===========================
		if m.state == stateConfirmDelete {
			switch key {
			case "y", "Y", "enter":
				deleted := 0
				var lastErr error

				for _, path := range m.confirmItems {
					if err := os.RemoveAll(path); err != nil {
						lastErr = err
						continue
					}
					deleted++
					delete(m.selected, path)
					m.removeFromCut(path)
				}

				m.confirmItems = nil
				m.state = stateNormal
				m.loadEntries()

				if deleted > 0 {
					m.status = fmt.Sprintf("Deleted %d item(s).", deleted)
				} else if lastErr != nil {
					m.status = "Delete failed: " + lastErr.Error()
				} else {
					m.status = "Nothing deleted"
				}

				return m, nil

			case "n", "N", "esc":
				m.state = stateNormal
				m.confirmItems = nil
				m.status = "Deletion cancelled"
				return m, nil
			}
		}

		// ===========================
		// NORMAL MODE
		// ===========================
		switch key {

		case "q", "ctrl+c":
			return m, tea.Quit

		case "j", "down":
			m.moveCursor(1)

		case "k", "up":
			m.moveCursor(-1)

		case "enter", "l":
			if len(m.entries) == 0 {
				return m, nil
			}
			entry := m.entries[m.cursor]
			full := filepath.Join(m.cwd, entry.Name())

			if entry.IsDir() {
				m.cwd = full
				m.loadEntries()
				m.status = ""
			} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
				_ = exec.Command("zathura", full).Start()
			} else {
				m.status = "Not a PDF"
			}

		case "h", "backspace":
			parent := filepath.Dir(m.cwd)

			if parent == m.cwd || !strings.HasPrefix(parent, m.root) {
				m.status = "Already at root"
				return m, nil
			}

			m.cwd = parent
			m.loadEntries()
			m.status = ""

		case "g":
			m.cursor = 0; m.ensureCursorVisible()
		case "G":
			if n := len(m.entries); n > 0 {
				m.cursor = n-1; m.ensureCursorVisible()
			}

		// case "R":
		// 	m.loadEntries()
		// 	m.status = "Reloaded"

		case " ":
			if len(m.entries) == 0 {
				return m, nil
			}

			full := filepath.Join(m.cwd, m.entries[m.cursor].Name())

			// toggle
			if m.selected[full] {
				delete(m.selected, full)
			} else {
				m.selected[full] = true
			}

			m.moveCursor(1)
			// // step down and keep viewport in sync
			// if m.cursor < len(m.entries)-1 {
			// 	m.cursor++
			// 	m.ensureCursorVisible()
			// }

			return m, nil

		case "d":
			targets := m.selectionOrCurrent()
			if len(targets) == 0 {
				m.status = "Nothing to cut"
				return m, nil
			}

			m.cut = append([]string{}, targets...)
			for _, t := range targets {
				delete(m.selected, t)
			}

			m.status = fmt.Sprintf("Cut %d item(s). Paste with 'p'.", len(targets))

		case "p":
			if len(m.cut) == 0 {
				m.status = "Cut buffer empty"
				return m, nil
			}

			moved := 0
			var lastErr error

			for _, src := range m.cut {
				dst := filepath.Join(m.cwd, filepath.Base(src))
				dst = avoidNameClash(dst)

				if err := os.Rename(src, dst); err != nil {
					lastErr = err
					continue
				}
				moved++
			}

			m.cut = nil
			m.loadEntries()

			if moved > 0 {
				m.status = fmt.Sprintf("Moved %d item(s).", moved)
			} else if lastErr != nil {
				m.status = "Move failed: " + lastErr.Error()
			}

		case "D":
			targets := m.selectionOrCurrent()
			if len(targets) == 0 {
				m.status = "Nothing to delete"
				return m, nil
			}

			m.confirmItems = targets
			m.state = stateConfirmDelete

			if len(targets) == 1 {
				m.status = fmt.Sprintf("Delete '%s'? (y/N)", filepath.Base(targets[0]))
			} else {
				m.status = fmt.Sprintf("Delete %d items? (y/N)", len(targets))
			}

		case "a":
			m.state = stateNewDir
			m.input.SetValue("")
			m.status = "New directory: type name and press Enter"
		}
	}

	return m, nil
}

func (m *model) removeFromCut(path string) {
	out := m.cut[:0]
	for _, c := range m.cut {
		if c != path {
			out = append(out, c)
		}
	}
	m.cut = out
}

func (m model) selectionOrCurrent() []string {
	if len(m.selected) > 0 {
		out := make([]string, 0, len(m.selected))
		for p := range m.selected {
			out = append(out, p)
		}
		return out
	}
	if len(m.entries) == 0 {
		return nil
	}
	full := filepath.Join(m.cwd, m.entries[m.cursor].Name())
	return []string{full}
}

func avoidNameClash(dst string) string {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}
	ext := filepath.Ext(dst)
	base := strings.TrimSuffix(filepath.Base(dst), ext)
	dir := filepath.Dir(dst)

	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func (m model) View() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Dir : %s\n\n", m.cwd)

	// viewport range
	end := m.viewportStart + m.viewportHeight
	if end > len(m.entries) {
		end = len(m.entries)
	}

	if len(m.entries) == 0 {
		b.WriteString("(empty)\n")
	} else {
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

			if e.IsDir() {
				fmt.Fprintf(&b, "%s%s📁 %s/\n", cursor, sel, e.Name())
			} else {
				fmt.Fprintf(&b, "%s%s📄 %s\n", cursor, sel, e.Name())
			}
		}
	}


    // Show footer only when not in popup
    if m.state != stateNewDir {
        b.WriteString("\n[j/k] move  [l] enter  [h] up  [space] select  [d] cut  [p] paste  [a] mkdir  [D] delete  [q] quit\n")
        if m.status != "" {
            b.WriteString("\n" + m.status + "\n")
        }
        return b.String()
    }

    // -------------- POPUP MODE --------------

    popupContent := fmt.Sprintf(
        " Create Directory \n\n %s \n\n (enter to confirm, esc to cancel) ",
        m.input.View(),
    )

    popup := popupBox(40, 6, popupContent)

    // Center the popup
    lines := strings.Split(b.String(), "\n")
    var final strings.Builder

    for _, line := range lines {
        final.WriteString(line + "\n")
    }

    final.WriteString("\n\n")
    final.WriteString(popup)

    return final.String()

	// // footer
	// if m.state == stateNewDir {
	// 	fmt.Fprintf(&b, "\nCreate directory: %s\n", m.input.View())
	// } else {
	// 	b.WriteString("\n[j/k] move  [l/enter] enter/open  [h/backspace] up  [space] select  [d] cut  [p] paste  [a] mkdir  [D] delete  [q] quit\n")
	// }

	// if m.status != "" {
	// 	fmt.Fprintf(&b, "\n%s\n", m.status)
	// }

	// return b.String()
}

func popupBox(width, height int, content string) string {
    lines := strings.Split(content, "\n")

    // pad lines to box width
    for i := range lines {
        lines[i] = " " + lines[i] + strings.Repeat(" ", width-len(lines[i])-1)
    }

    top := "┌" + strings.Repeat("─", width-2) + "┐"
    bottom := "└" + strings.Repeat("─", width-2) + "┘"

    var b strings.Builder
    b.WriteString(top + "\n")
    for _, line := range lines {
        b.WriteString("│" + line + "│\n")
    }
    b.WriteString(bottom)

    return b.String()
}

func main() {
    root := flag.String("root", ".", "Root directory to start in")
    flag.Parse()

    p := tea.NewProgram(
        newModel(*root),
        tea.WithAltScreen(),   // <— Put the TUI in full-screen mode
    )

    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}
