package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}

		case "g":
			m.cursor = 0; m.ensureCursorVisible()
		case "G":
			if n := len(m.entries); n > 0 {
				m.cursor = n-1; m.ensureCursorVisible()
			}

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

			// MOVE CURSOR DOWN & keep visible
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}

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
