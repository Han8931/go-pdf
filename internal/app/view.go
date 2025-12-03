package app

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (m Model) View() string {
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

	// footer
	if m.state == stateNewDir {
		fmt.Fprintf(&b, "\nCreate directory: %s\n", m.input.View())
	} else {
		b.WriteString("\n[j/k] move  [l/enter] enter/open  [h/backspace] up  [space] select  [d] cut  [p] paste  [a] mkdir  [D] delete  [q] quit\n")
	}

	if m.status != "" {
		fmt.Fprintf(&b, "\n%s\n", m.status)
	}

	return b.String()
}
