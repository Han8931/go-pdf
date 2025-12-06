package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gorae/internal/arxiv"
	"gorae/internal/config"
	"gorae/internal/meta"
)

type configEditFinishedMsg struct {
	err error
}

type metadataEditFinishedMsg struct {
	err        error
	tmpPath    string
	targetPath string
}

type arxivUpdateMsg struct {
	arxivID      string
	updatedPaths []string
	err          error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.viewportHeight = msg.Height - 5
		if m.viewportHeight < 1 {
			m.viewportHeight = 1
		}
		m.width = msg.Width
		m.ensureCursorVisible()
		return m, nil

	case configEditFinishedMsg:
		if msg.err != nil {
			m.setStatus("Config edit failed: " + msg.err.Error())
		} else {
			m.setStatus("Config edit finished")
		}
		return m, nil

	case metadataEditFinishedMsg:
		m.handleMetadataEditorFinished(msg)
		return m, nil

	case arxivUpdateMsg:
		if msg.err != nil {
			m.setStatus("arXiv import failed: " + msg.err.Error())
			return m, nil
		}
		if len(msg.updatedPaths) > 0 {
			current := m.currentEntryPath()
			m.resortAndPreserveSelection()
			for _, path := range msg.updatedPaths {
				if m.currentMetaPath == path {
					m.currentMetaPath = ""
					m.updateCurrentMetadata(path)
				}
				if current != "" && path == current {
					m.updateTextPreview()
				}
			}
		}
		count := len(msg.updatedPaths)
		if count == 0 {
			m.setStatus("arXiv import completed, but no files were updated")
		} else {
			m.setStatus(fmt.Sprintf("arXiv %s metadata applied to %d file(s)", msg.arxivID, count))
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		if m.state != stateCommand && len(m.commandOutput) > 0 {
			m.clearCommandOutput()
		}

		if m.state != stateNormal && m.awaitingSort {
			m.awaitingSort = false
		}

		if m.state == stateNormal && m.awaitingSort {
			m.awaitingSort = false
			switch strings.ToLower(key) {
			case "t":
				m.applySortMode(sortByTitle)
			case "y":
				m.applySortMode(sortByYear)
			default:
				m.setStatus("Sort cancelled")
			}
			return m, nil
		}

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
					m.setStatus("Directory name cannot be empty")
					return m, cmd
				}
				if strings.HasPrefix(name, ".") {
					m.setStatus("Dot directories are hidden; choose another name")
					return m, cmd
				}

				dst := filepath.Join(m.cwd, name)
				if _, err := os.Stat(dst); err == nil {
					m.setStatus("Already exists")
					return m, cmd
				}

				if err := os.MkdirAll(dst, 0o755); err != nil {
					m.setStatus("Failed: " + err.Error())
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
				m.setStatus("Directory created")
				return m, cmd

			case "esc":
				m.state = stateNormal
				m.setStatus("Cancelled")
				m.input.SetValue("")
				return m, cmd
			}

			return m, cmd
		}

		// ===========================
		//  RENAME MODE
		// ===========================
		if m.state == stateRename {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)

			switch key {
			case "enter":
				newName := strings.TrimSpace(m.input.Value())
				oldPath := m.renameTarget

				m.state = stateNormal
				m.input.SetValue("")
				m.renameTarget = ""

				if newName == "" {
					m.setStatus("Name cannot be empty")
					return m, cmd
				}

				if strings.Contains(newName, "/") {
					m.setStatus("Name cannot contain '/'")
					return m, cmd
				}

				dir := filepath.Dir(oldPath)
				newPath := filepath.Join(dir, newName)

				if _, err := os.Stat(newPath); err == nil {
					m.setStatus("Target already exists")
					return m, cmd
				}

				if err := os.Rename(oldPath, newPath); err != nil {
					m.setStatus("Rename failed: " + err.Error())
					return m, cmd
				}

				var metaErr error
				if err := m.moveMetadataPaths(oldPath, newPath, true); err != nil {
					metaErr = err
				}

				m.loadEntries()
				for i, e := range m.entries {
					if e.Name() == newName {
						m.cursor = i
						break
					}
				}
				m.ensureCursorVisible()
				m.updateTextPreview()
				if metaErr != nil {
					m.setStatus("Renamed, but metadata update failed: " + metaErr.Error())
				} else {
					m.setStatus("Renamed")
				}
				return m, cmd

			case "esc":
				m.state = stateNormal
				m.input.SetValue("")
				m.renameTarget = ""
				m.setStatus("Rename cancelled")
				return m, cmd
			}

			return m, cmd
		}

		// ===========================
		//  EDIT METADATA MODE
		// ===========================
		if m.state == stateMetaPreview {
			switch key {
			case "e":
				m.state = stateEditMeta
				m.metaFieldIndex = 0
				m.loadMetaFieldIntoInput()
				m.input.Focus()
				m.setPersistentStatus(metaEditStatus(m.metaFieldIndex))
				return m, nil
			case "v":
				if cmd := m.launchMetadataEditor(); cmd != nil {
					return m, cmd
				}
				return m, nil
			case "esc":
				m.state = stateNormal
				m.metaEditingPath = ""
				m.setStatus("Metadata edit cancelled")
				return m, nil
			}
			return m, nil
		}

		if m.state == stateEditMeta {
			var cmd tea.Cmd
			if key != "tab" && key != "shift+tab" {
				m.input, cmd = m.input.Update(msg)
			}

			switch key {
			case "tab":
				val := strings.TrimSpace(m.input.Value())
				setMetadataFieldValue(&m.metaDraft, m.metaFieldIndex, val)
				if m.metaFieldIndex < metaFieldCount()-1 {
					m.metaFieldIndex++
				}
				m.loadMetaFieldIntoInput()
				m.setPersistentStatus(metaEditStatus(m.metaFieldIndex))
				return m, cmd

			case "shift+tab":
				val := strings.TrimSpace(m.input.Value())
				setMetadataFieldValue(&m.metaDraft, m.metaFieldIndex, val)
				if m.metaFieldIndex > 0 {
					m.metaFieldIndex--
				}
				m.loadMetaFieldIntoInput()
				m.setPersistentStatus(metaEditStatus(m.metaFieldIndex))
				return m, cmd

			case "enter":
				val := strings.TrimSpace(m.input.Value())
				setMetadataFieldValue(&m.metaDraft, m.metaFieldIndex, val)

				if m.metaFieldIndex < metaFieldCount()-1 {
					m.metaFieldIndex++
					m.loadMetaFieldIntoInput()
					m.setPersistentStatus(metaEditStatus(m.metaFieldIndex))
					return m, cmd
				}

				if m.metaDraft.Path == "" {
					m.metaDraft.Path = m.metaEditingPath
				}
				if m.meta != nil {
					ctx := context.Background()
					if err := m.meta.Upsert(ctx, &m.metaDraft); err != nil {
						m.setStatus("Failed to save metadata: " + err.Error())
					} else {
						m.setStatus("Metadata saved")
						m.currentMetaPath = ""
						m.resortAndPreserveSelection()
					}
				} else {
					m.setStatus("Metadata store not available")
				}
				m.state = stateNormal
				m.input.SetValue("")
				m.metaEditingPath = ""
				return m, cmd

			case "esc":
				m.state = stateNormal
				m.input.SetValue("")
				m.metaEditingPath = ""
				m.setStatus("Metadata edit cancelled")
				return m, cmd
			}

			return m, cmd
		}

		// ===========================
		//  COMMAND MODE
		// ===========================
		if m.state == stateCommand {
			if key == "tab" {
				if m.handleCommandAutocomplete() {
					return m, nil
				}
			}
			var inputCmd tea.Cmd
			m.input, inputCmd = m.input.Update(msg)

			switch key {
			case "enter":
				line := m.input.Value()
				m.state = stateNormal
				m.input.SetValue("")
				m.input.Blur()
				cmd := m.runCommand(line)
				return m, tea.Batch(inputCmd, cmd)
			case "esc":
				m.state = stateNormal
				m.input.SetValue("")
				m.input.Blur()
				m.setStatus("Command cancelled")
				return m, inputCmd
			default:
				return m, inputCmd
			}
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
					m.setStatus(fmt.Sprintf("Deleted %d item(s).", deleted))
				} else if lastErr != nil {
					m.setStatus("Delete failed: " + lastErr.Error())
				} else {
					m.setStatus("Nothing deleted")
				}

				return m, nil

			case "n", "N", "esc":
				m.state = stateNormal
				m.confirmItems = nil
				m.setStatus("Deletion cancelled")
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
				m.updateTextPreview() // <── NEW
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
				m.updateTextPreview()
			}

		case "g":
			m.cursor = 0
			m.ensureCursorVisible()

		case "G":
			if n := len(m.entries); n > 0 {
				m.cursor = n - 1
				m.ensureCursorVisible()
			}

		// case "enter", "l":
		// 	if len(m.entries) == 0 {
		// 		return m, nil
		// 	}
		// 	entry := m.entries[m.cursor]
		// 	full := filepath.Join(m.cwd, entry.Name())

		// 	if entry.IsDir() {
		// 		m.cwd = full
		// 		m.loadEntries()
		// 		m.status = ""
		// 	} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
		// 		_ = exec.Command("zathura", full).Start()
		// 	} else {
		// 		m.status = "Not a PDF"
		// 	}

		case "enter", "l":
			if len(m.entries) == 0 {
				return m, nil
			}
			entry := m.entries[m.cursor]
			full := filepath.Join(m.cwd, entry.Name())

			if entry.IsDir() {
				m.cwd = full
				m.loadEntries()
				m.clearStatus()
				m.updateTextPreview() // <── NEW
			} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
				if err := exec.Command("zathura", full).Start(); err != nil {
					m.setStatus("Failed to open PDF: " + err.Error())
				} else {
					m.recordRecentlyOpened(full)
				}
			} else {
				m.setStatus("Not a PDF")
			}

		case "h", "backspace":
			parent := filepath.Dir(m.cwd)

			if parent == m.cwd || !strings.HasPrefix(parent, m.root) {
				m.setStatus("Already at root")
				return m, nil
			}

			m.cwd = parent
			m.loadEntries()
			m.clearStatus()
			m.updateTextPreview() // <── NEW

		case "s":
			m.awaitingSort = true
			m.setStatus("Sort: 't' by title, 'y' by year")
			return m, nil

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
				m.setStatus("Nothing to cut")
				return m, nil
			}

			m.cut = append([]string{}, targets...)
			for _, t := range targets {
				delete(m.selected, t)
			}

			m.setStatus(fmt.Sprintf("Cut %d item(s). Paste with 'p'.", len(targets)))

		case "p":
			if len(m.cut) == 0 {
				m.setStatus("Cut buffer empty")
				return m, nil
			}

			moved := 0
			var lastErr error
			var metaErr error

			for _, src := range m.cut {
				info, err := os.Stat(src)
				if err != nil {
					lastErr = err
					continue
				}

				dst := filepath.Join(m.cwd, filepath.Base(src))
				dst = avoidNameClash(dst)

				if err := os.Rename(src, dst); err != nil {
					lastErr = err
					continue
				}

				if err := m.moveMetadataPaths(src, dst, info.IsDir()); err != nil {
					metaErr = err
				}

				moved++
			}

			m.cut = nil
			m.loadEntries()
			m.updateTextPreview()

			if moved > 0 {
				msg := fmt.Sprintf("Moved %d item(s).", moved)
				if metaErr != nil {
					msg += " Metadata update failed: " + metaErr.Error()
				}
				m.setStatus(msg)
			} else if lastErr != nil {
				m.setStatus("Move failed: " + lastErr.Error())
			} else if metaErr != nil {
				m.setStatus("Metadata update failed: " + metaErr.Error())
			}

		case "D":
			targets := m.selectionOrCurrent()
			if len(targets) == 0 {
				m.setStatus("Nothing to delete")
				return m, nil
			}

			m.confirmItems = targets
			m.state = stateConfirmDelete

			if len(targets) == 1 {
				m.setStatus(fmt.Sprintf("Delete '%s'? (y/N)", filepath.Base(targets[0])))
			} else {
				m.setStatus(fmt.Sprintf("Delete %d items? (y/N)", len(targets)))
			}

		case "r":
			if len(m.entries) == 0 {
				m.setStatus("Nothing to rename")
				return m, nil
			}

			entry := m.entries[m.cursor]
			full := filepath.Join(m.cwd, entry.Name())

			if !entry.IsDir() {
				m.setStatus("Not a directory")
				return m, nil
			}

			m.state = stateRename
			m.renameTarget = full
			m.input.SetValue(entry.Name())
			m.input.CursorEnd() // put cursor at end
			m.input.Focus()
			m.setPersistentStatus("Rename: edit name and press Enter")
			return m, nil

		case "a":
			m.state = stateNewDir
			m.input.SetValue("")
			m.input.CursorEnd()
			m.input.Focus()
			m.setPersistentStatus("New directory: type name and press Enter")

		case "e":
			if len(m.entries) == 0 {
				m.setStatus("Nothing to edit")
				return m, nil
			}

			entry := m.entries[m.cursor]
			full := filepath.Join(m.cwd, entry.Name())
			info, err := entry.Info()
			isDir := entry.IsDir()
			if err == nil {
				isDir = info.IsDir()
			}

			// For now: only files (skip dirs)
			if isDir {
				m.setStatus("Metadata editing is for files only")
				return m, nil
			}

			canonical := canonicalPath(full)

			m.state = stateMetaPreview
			m.metaEditingPath = canonical

			// load existing metadata if present
			draft := meta.Metadata{Path: canonical}
			if m.meta != nil {
				ctx := context.Background()
				existing, err := m.meta.Get(ctx, canonical)
				if err != nil {
					m.setStatus("Failed to load metadata: " + err.Error())
				} else if existing != nil {
					draft = *existing
				}
			}
			m.metaDraft = draft
			m.metaFieldIndex = 0
			m.input.SetValue("")
			m.input.Blur()
			m.setPersistentStatus("Metadata preview: 'e' edit here, 'v' open editor, Esc cancel")
			return m, nil

		case ":":
			m.state = stateCommand
			m.input.SetValue(":")
			m.input.CursorEnd()
			m.input.Focus()
			m.setPersistentStatus("Command mode (:help for list, Esc to cancel)")
			return m, nil

		}
	}

	return m, nil
}

func metaEditStatus(index int) string {
	label := metaFieldLabel(index)
	if index == metaFieldCount()-1 {
		return fmt.Sprintf("Edit %s (Enter to save, Tab/Shift+Tab to move, Esc to cancel)", label)
	}
	return fmt.Sprintf("Edit %s (Enter/Tab to continue, Shift+Tab to go back, Esc to cancel)", label)
}

func (m *Model) handleMetadataEditorFinished(msg metadataEditFinishedMsg) {
	if msg.tmpPath != "" {
		defer os.Remove(msg.tmpPath)
	}
	m.state = stateNormal
	m.metaEditingPath = ""
	m.input.SetValue("")

	if msg.err != nil {
		m.setStatus("Metadata editor failed: " + msg.err.Error())
		return
	}
	if strings.TrimSpace(msg.tmpPath) == "" {
		m.setStatus("Metadata editor failed: no data returned")
		return
	}
	target := strings.TrimSpace(msg.targetPath)
	if target == "" {
		m.setStatus("Metadata editor failed: unknown target")
		return
	}
	data, err := os.ReadFile(msg.tmpPath)
	if err != nil {
		m.setStatus("Failed to read metadata edit: " + err.Error())
		return
	}
	md, err := parseMetadataEditorData(data, target)
	if err != nil {
		m.setStatus("Failed to parse metadata: " + err.Error())
		return
	}
	if m.meta == nil {
		m.setStatus("Metadata store not available")
		return
	}
	ctx := context.Background()
	if err := m.meta.Upsert(ctx, &md); err != nil {
		m.setStatus("Failed to save metadata: " + err.Error())
		return
	}
	m.metaDraft = md
	m.currentMetaPath = ""
	m.resortAndPreserveSelection()
	m.setStatus("Metadata saved")
}

func (m *Model) launchMetadataEditor() tea.Cmd {
	target := strings.TrimSpace(m.metaEditingPath)
	if target == "" {
		m.setStatus("No metadata target selected")
		return nil
	}
	if strings.TrimSpace(m.metaDraft.Path) == "" {
		m.metaDraft.Path = target
	}
	tmp, err := os.CreateTemp("", "gorae-metadata-*.json")
	if err != nil {
		m.setStatus("Failed to create temp file: " + err.Error())
		return nil
	}
	tmpPath := tmp.Name()
	data := metadataEditorFileFromMetadata(m.metaDraft)
	if err := writeMetadataEditorFile(tmp, data); err != nil {
		os.Remove(tmpPath)
		m.setStatus("Failed to prepare metadata for editor: " + err.Error())
		return nil
	}
	editor := m.configEditor()
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fileName := filepath.Base(target)
	if fileName == "" {
		fileName = target
	}
	m.state = stateNormal
	m.metaEditingPath = ""
	m.input.SetValue("")
	m.setPersistentStatus(fmt.Sprintf("Editing metadata for %s with %s (exit editor to return)", fileName, editor))

	targetPath := target
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return metadataEditFinishedMsg{
			err:        err,
			tmpPath:    tmpPath,
			targetPath: targetPath,
		}
	})
}

type metadataEditorFile struct {
	Title    string `json:"title"`
	Author   string `json:"author"`
	Venue    string `json:"venue"`
	Year     string `json:"year"`
	Abstract string `json:"abstract"`
}

func metadataEditorFileFromMetadata(md meta.Metadata) metadataEditorFile {
	return metadataEditorFile{
		Title:    md.Title,
		Author:   md.Author,
		Venue:    md.Venue,
		Year:     md.Year,
		Abstract: md.Abstract,
	}
}

func writeMetadataEditorFile(f *os.File, data metadataEditorFile) error {
	defer f.Close()
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		return err
	}
	if _, err := f.WriteString("\n"); err != nil {
		return err
	}
	return nil
}

func parseMetadataEditorData(raw []byte, path string) (meta.Metadata, error) {
	var data metadataEditorFile
	if err := json.Unmarshal(raw, &data); err != nil {
		return meta.Metadata{}, fmt.Errorf("parse JSON: %w", err)
	}
	md := meta.Metadata{
		Path:     path,
		Title:    strings.TrimSpace(data.Title),
		Author:   strings.TrimSpace(data.Author),
		Venue:    strings.TrimSpace(data.Venue),
		Year:     strings.TrimSpace(data.Year),
		Abstract: strings.TrimSpace(data.Abstract),
	}
	return md, nil
}

func (m *Model) moveMetadataPaths(oldPath, newPath string, isDir bool) error {
	if m.meta == nil {
		return nil
	}
	ctx := context.Background()
	if isDir {
		if err := m.meta.MoveTree(ctx, oldPath, newPath); err != nil {
			return err
		}
		return m.meta.MovePath(ctx, oldPath, newPath)
	}
	return m.meta.MovePath(ctx, oldPath, newPath)
}

func (m *Model) runCommand(raw string) tea.Cmd {
	text := strings.TrimSpace(raw)
	if text == "" {
		m.setStatus("No command entered")
		return nil
	}
	if strings.HasPrefix(text, ":") {
		text = strings.TrimSpace(text[1:])
	}
	if text == "" {
		m.setStatus("No command entered")
		return nil
	}

	fields := strings.Fields(text)
	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "h", "help":
		help := []string{
			"Command Help:",
			"  Navigation : j/k move, h up, l enter",
			"  Selection  : space toggle, d cut, p paste",
			"  Files      : a mkdir, r rename dir, D delete",
			"  Metadata   : e preview/edit metadata, v edit metadata in editor, :arxiv <id> [files...] fetch from arXiv",
			"  Recently Added : :recent rebuilds the Recently Added directory (names show metadata titles when available)",
			"  Recently Opened: open a PDF to refresh the Recently Opened directory (keeps last 20)",
			"  Config     : :config shows/edits the config file",
			"  Commands   : :h help, :pwd show directory, :clear hide pane, :q quit",
		}
		m.setCommandOutput(help)
		m.setPersistentStatus("Help displayed (use :clear to hide)")
	case "pwd":
		output := []string{
			"Current directory:",
			"  " + m.cwd,
		}
		m.setCommandOutput(output)
		m.setStatus("Printed working directory")
	case "clear":
		m.clearCommandOutput()
		m.setStatus("Command output cleared")
	case "recent":
		if err := m.maybeSyncRecentlyAddedDir(true); err != nil {
			m.setStatus("Recently added sync failed: " + err.Error())
		} else {
			m.setStatus("Recently added directory updated")
		}
	case "config":
		return m.handleConfigCommand(args)
	case "arxiv":
		return m.handleArxivCommand(args)
	case "q", "quit":
		m.setStatus("Quitting...")
		return tea.Quit
	default:
		if len(args) > 0 {
			m.setStatus(fmt.Sprintf("Unknown command: %s (args: %s)", cmd, strings.Join(args, " ")))
		} else {
			m.setStatus(fmt.Sprintf("Unknown command: %s", cmd))
		}
	}
	return nil
}

func (m *Model) handleConfigCommand(args []string) tea.Cmd {
	if len(args) == 0 {
		path, err := config.Path()
		if err != nil {
			m.setStatus("Failed to resolve config path: " + err.Error())
			return nil
		}
		editor := m.configEditor()
		lines := []string{
			"Config file:",
			"  " + path,
			"Configured editor:",
			"  " + editor,
			"Use :config edit to open it.",
		}
		m.setCommandOutput(lines)
		m.setPersistentStatus("Config path displayed (use :clear to hide)")
		return nil
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "edit":
		path, err := config.Path()
		if err != nil {
			m.setStatus("Failed to resolve config path: " + err.Error())
			return nil
		}
		editor := m.configEditor()
		cmd := exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		m.setPersistentStatus(fmt.Sprintf("Editing config with %s (exit editor to return)", editor))
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return configEditFinishedMsg{err: err}
		})
	default:
		m.setStatus(fmt.Sprintf("Unknown config command: %s", sub))
		return nil
	}
}

func (m *Model) configEditor() string {
	if m.cfg != nil {
		if editor := strings.TrimSpace(m.cfg.Editor); editor != "" {
			return editor
		}
	}
	if env := strings.TrimSpace(os.Getenv("EDITOR")); env != "" {
		return env
	}
	return "vi"
}

func (m *Model) handleArxivCommand(args []string) tea.Cmd {
	if m.meta == nil {
		m.setStatus("Metadata store not available")
		return nil
	}
	if len(args) == 0 {
		m.setStatus("Usage: :arxiv <arxiv-id> [files...]")
		return nil
	}
	id := args[0]
	var files []string
	if len(args) > 1 {
		fileArgs := args[1:]
		files = make([]string, 0, len(fileArgs))
		for _, spec := range fileArgs {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			resolved, err := m.resolveCommandFilePath(spec)
			if err != nil {
				m.setStatus(err.Error())
				return nil
			}
			files = append(files, resolved)
		}
	} else {
		targets := m.selectionOrCurrent()
		if len(targets) == 0 {
			m.setStatus("No files selected")
			return nil
		}
		files = make([]string, 0, len(targets))
		for _, path := range targets {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.IsDir() {
				continue
			}
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		m.setStatus("arXiv import works on files only; specify or select at least one PDF")
		return nil
	}
	files = uniquePaths(files)
	m.setPersistentStatus(fmt.Sprintf("Fetching arXiv %s for %d file(s)...", id, len(files)))
	return m.fetchArxivMetadata(id, files)
}

func (m *Model) fetchArxivMetadata(id string, files []string) tea.Cmd {
	store := m.meta
	paths := append([]string{}, files...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		metadata, err := arxiv.Fetch(ctx, id)
		if err != nil {
			return arxivUpdateMsg{err: err}
		}

		authorStr := strings.Join(metadata.Authors, ", ")
		yearStr := ""
		if metadata.Year > 0 {
			yearStr = strconv.Itoa(metadata.Year)
		}

		baseCtx := context.Background()
		updated := make([]string, 0, len(paths))
		for _, path := range paths {
			existing, err := store.Get(baseCtx, path)
			if err != nil {
				return arxivUpdateMsg{err: fmt.Errorf("load metadata for %s: %w", filepath.Base(path), err)}
			}
			md := meta.Metadata{Path: path}
			if existing != nil {
				md = *existing
			}
			md.Title = metadata.Title
			md.Author = authorStr
			md.Year = yearStr
			md.Abstract = metadata.Abstract
			if err := store.Upsert(baseCtx, &md); err != nil {
				return arxivUpdateMsg{err: fmt.Errorf("save metadata for %s: %w", filepath.Base(path), err)}
			}
			updated = append(updated, path)
		}

		return arxivUpdateMsg{arxivID: metadata.ID, updatedPaths: updated}
	}
}

type pathCompletion struct {
	value string
	isDir bool
}

func (m *Model) resolveCommandFilePath(spec string) (string, error) {
	resolved := spec
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(m.cwd, spec)
	}
	resolved = filepath.Clean(resolved)
	if !strings.HasPrefix(resolved, m.root) {
		return "", fmt.Errorf("Path not under root: %s", spec)
	}
	info, err := os.Stat(resolved)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("Cannot fetch arXiv metadata for directory: %s", spec)
		}
		return resolved, nil
	}
	if filepath.Ext(resolved) != "" {
		return "", fmt.Errorf("File not found: %s", spec)
	}
	dir := filepath.Dir(resolved)
	base := filepath.Base(resolved)
	entries, dirErr := os.ReadDir(dir)
	if dirErr != nil {
		return "", fmt.Errorf("File not found: %s", spec)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if !strings.EqualFold(ext, ".pdf") {
			continue
		}
		if strings.EqualFold(strings.TrimSuffix(name, ext), base) {
			full := filepath.Join(dir, name)
			info, statErr := os.Stat(full)
			if statErr == nil && !info.IsDir() {
				return full, nil
			}
		}
	}
	return "", fmt.Errorf("File not found: %s", spec)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func (m *Model) handleCommandAutocomplete() bool {
	if m.state != stateCommand {
		return false
	}
	value := m.input.Value()
	runes := []rune(value)
	cursor := m.input.Position()
	if cursor != len(runes) {
		return false
	}
	current := string(runes[:cursor])
	trimmed := strings.TrimRight(current, " \t")
	if !strings.ContainsAny(trimmed, " \t") {
		return false
	}
	lastSep := strings.LastIndexAny(trimmed, " \t")
	if lastSep == -1 || lastSep == len(trimmed)-1 {
		return false
	}
	token := trimmed[lastSep+1:]
	if token == "" {
		return false
	}
	completions := m.commandPathCompletions(token)
	if len(completions) == 0 {
		m.setStatus("No completions")
		return true
	}
	values := make([]string, len(completions))
	for i, c := range completions {
		values[i] = c.value
	}
	lcp := longestCommonPrefix(values)
	if lcp == token {
		lines := []string{"Completions:"}
		for _, c := range completions {
			lines = append(lines, "  "+c.value)
		}
		m.setCommandOutput(lines)
		m.setPersistentStatus("Multiple completions (type more letters)")
		return true
	}
	appendSpace := false
	if len(completions) == 1 && lcp == completions[0].value && !completions[0].isDir {
		appendSpace = true
	}
	prefix := trimmed[:lastSep+1]
	newValue := prefix + lcp
	if appendSpace {
		newValue += " "
	}
	m.input.SetValue(newValue)
	m.input.CursorEnd()
	return true
}

func (m *Model) commandPathCompletions(token string) []pathCompletion {
	if token == "" {
		return nil
	}
	if filepath.IsAbs(token) {
		return nil
	}
	dirPart, partial := filepath.Split(token)
	searchDir := m.cwd
	origDirPart := dirPart
	if dirPart != "" {
		candidate := filepath.Join(m.cwd, dirPart)
		candidate = filepath.Clean(candidate)
		if !strings.HasPrefix(candidate, m.root) {
			return nil
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			return nil
		}
		searchDir = candidate
	}
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}
	sep := string(os.PathSeparator)
	comps := make([]pathCompletion, 0)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, partial) {
			continue
		}
		display := name
		if !e.IsDir() {
			ext := filepath.Ext(name)
			if strings.EqualFold(ext, ".pdf") {
				display = strings.TrimSuffix(name, ext)
			}
			if display == "" {
				display = name
			}
		}
		completion := display
		if origDirPart != "" {
			base := strings.TrimSuffix(origDirPart, sep)
			if base == "" {
				completion = display
			} else {
				completion = filepath.Join(base, display)
			}
		}
		if e.IsDir() && !strings.HasSuffix(completion, sep) {
			completion += sep
		}
		comps = append(comps, pathCompletion{value: completion, isDir: e.IsDir()})
	}
	sort.Slice(comps, func(i, j int) bool {
		return comps[i].value < comps[j].value
	})
	return comps
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if prefix == "" {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}
