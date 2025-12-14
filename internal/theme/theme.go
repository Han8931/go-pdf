package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorae/internal/simpletoml"
)

const defaultThemeFile = `# Gorae color theme.
# Update the values below to customize the UI.

[meta]
name = "Gorae Default"
version = 1

[palette]
bg = "#1e1e2e"
fg = "#f2d5cf"
muted = "#7f8ca3"
accent = "#cba6f7"
success = "#a6e3a1"
warning = "#f9e2af"
danger = "#f38ba8"
selection = "#89dceb"

[borders]
style = "rounded"
color = "#585b70"

[icons]
mode = "unicode"
favorite = "★"
toread = "•"
read = "✓"
reading = "▶"
unread = "○"
folder = "▸"
pdf = "▣"
selected = "✔"
selection = "▌"

[components.app_header]
fg = "#f5e0dc"
bg = "#1e1e2e"
bold = true

[components.tree_header]
fg = "#a6e3a1"
bg = "#1b2430"
bold = true

[components.tree_body]
fg = "#b4c2f8"

[components.tree_active]
fg = "#f9e2af"
bold = true

[components.tree_info]
fg = "#7f8ca3"
italic = true

[components.list_header]
fg = "#f5e0dc"
bg = "#2a2438"
bold = true

[components.list_body]
fg = "#f2d5cf"

[components.list_selected]
fg = "#89dceb"
bold = true

[components.list_cursor]
fg = "#1e1e2e"
bg = "#f9e2af"
bold = true

[components.list_cursor_selected]
fg = "#1e1e2e"
bg = "#fab387"
bold = true

[components.preview_header]
fg = "#cba6f7"
bg = "#241f3d"
bold = true

[components.preview_body]
fg = "#cdd6f4"

[components.separator]
fg = "#585b70"

[components.status_bar]
fg = "#cdd6f4"
bg = "#11111b"

[components.status_label]
fg = "#94e2d5"
bold = true

[components.status_value]
fg = "#f9e2af"

[components.prompt_label]
fg = "#1e1e2e"
bg = "#cba6f7"
bold = true

[components.prompt_value]
fg = "#f5e0dc"
bg = "#1e1e2e"

[components.meta_overlay]
fg = "#f2cdcd"
bg = "#312244"`

type Meta struct {
	Name    string `toml:"name"`
	Version int    `toml:"version"`
}

type Palette struct {
	BG        string `toml:"bg"`
	FG        string `toml:"fg"`
	Muted     string `toml:"muted"`
	Accent    string `toml:"accent"`
	Success   string `toml:"success"`
	Warning   string `toml:"warning"`
	Danger    string `toml:"danger"`
	Selection string `toml:"selection"`
}

type Borders struct {
	Style string `toml:"style"`
	Color string `toml:"color"`
}

type StyleSpec struct {
	FG     string `toml:"fg"`
	BG     string `toml:"bg"`
	Bold   bool   `toml:"bold"`
	Italic bool   `toml:"italic"`
	Faint  bool   `toml:"faint"`
}

type ComponentStyles struct {
	AppHeader        StyleSpec `toml:"app_header"`
	TreeHeader       StyleSpec `toml:"tree_header"`
	TreeBody         StyleSpec `toml:"tree_body"`
	TreeActive       StyleSpec `toml:"tree_active"`
	TreeInfo         StyleSpec `toml:"tree_info"`
	ListHeader       StyleSpec `toml:"list_header"`
	ListBody         StyleSpec `toml:"list_body"`
	ListSelected     StyleSpec `toml:"list_selected"`
	ListCursor       StyleSpec `toml:"list_cursor"`
	ListCursorSelect StyleSpec `toml:"list_cursor_selected"`
	PreviewHeader    StyleSpec `toml:"preview_header"`
	PreviewBody      StyleSpec `toml:"preview_body"`
	Separator        StyleSpec `toml:"separator"`
	StatusBar        StyleSpec `toml:"status_bar"`
	StatusLabel      StyleSpec `toml:"status_label"`
	StatusValue      StyleSpec `toml:"status_value"`
	PromptLabel      StyleSpec `toml:"prompt_label"`
	PromptValue      StyleSpec `toml:"prompt_value"`
	MetaOverlay      StyleSpec `toml:"meta_overlay"`
}

type Icons struct {
	Mode      string `toml:"mode"`
	Favorite  string `toml:"favorite"`
	ToRead    string `toml:"toread"`
	Read      string `toml:"read"`
	Reading   string `toml:"reading"`
	Unread    string `toml:"unread"`
	Folder    string `toml:"folder"`
	PDF       string `toml:"pdf"`
	Selected  string `toml:"selected"`
	Selection string `toml:"selection"`
}

type IconSet struct {
	Favorite  string
	ToRead    string
	Read      string
	Reading   string
	Unread    string
	Folder    string
	PDF       string
	Selected  string
	Selection string
}

type Theme struct {
	Meta       Meta            `toml:"meta"`
	Palette    Palette         `toml:"palette"`
	Borders    Borders         `toml:"borders"`
	Icons      Icons           `toml:"icons"`
	Components ComponentStyles `toml:"components"`
}

func Default() Theme {
	return Theme{
		Meta: Meta{Name: "Gorae Default", Version: 1},
		Palette: Palette{
			BG:        "#1e1e2e",
			FG:        "#f2d5cf",
			Muted:     "#7f8ca3",
			Accent:    "#cba6f7",
			Success:   "#a6e3a1",
			Warning:   "#f9e2af",
			Danger:    "#f38ba8",
			Selection: "#89dceb",
		},
		Borders: Borders{
			Style: "rounded",
			Color: "#585b70",
		},
		Icons: Icons{
			Mode:      "unicode",
			Favorite:  "★",
			ToRead:    "•",
			Read:      "✓",
			Reading:   "▶",
			Unread:    "○",
			Folder:    "▸",
			PDF:       "▣",
			Selected:  "✔",
			Selection: "▌",
		},
		Components: ComponentStyles{
			AppHeader:  StyleSpec{FG: "#f5e0dc", BG: "#1e1e2e", Bold: true},
			TreeHeader: StyleSpec{FG: "#a6e3a1", BG: "#1b2430", Bold: true},
			TreeBody:   StyleSpec{FG: "#b4c2f8"},
			TreeActive: StyleSpec{FG: "#f9e2af", Bold: true},
			TreeInfo:   StyleSpec{FG: "#7f8ca3", Italic: true},

			ListHeader:       StyleSpec{FG: "#f5e0dc", BG: "#2a2438", Bold: true},
			ListBody:         StyleSpec{FG: "#f2d5cf"},
			ListSelected:     StyleSpec{FG: "#89dceb", Bold: true},
			ListCursor:       StyleSpec{FG: "#1e1e2e", BG: "#f9e2af", Bold: true},
			ListCursorSelect: StyleSpec{FG: "#1e1e2e", BG: "#fab387", Bold: true},

			PreviewHeader: StyleSpec{FG: "#cba6f7", BG: "#241f3d", Bold: true},
			PreviewBody:   StyleSpec{FG: "#cdd6f4"},

			Separator:   StyleSpec{FG: "#585b70"},
			StatusBar:   StyleSpec{FG: "#cdd6f4", BG: "#11111b"},
			StatusLabel: StyleSpec{FG: "#94e2d5", Bold: true},
			StatusValue: StyleSpec{FG: "#f9e2af"},
			PromptLabel: StyleSpec{FG: "#1e1e2e", BG: "#cba6f7", Bold: true},
			PromptValue: StyleSpec{FG: "#f5e0dc", BG: "#1e1e2e"},
			MetaOverlay: StyleSpec{FG: "#f2cdcd", BG: "#312244"},
		},
	}
}

func LoadActive() (Theme, error) {
	path, err := themePath()
	if err != nil {
		return Theme{}, err
	}
	base := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := ensureDefaultTheme(path); err != nil {
				return base, err
			}
			data, err = os.ReadFile(path)
			if err != nil {
				return base, err
			}
		} else {
			return base, err
		}
	}
	if err := simpletoml.Decode(data, &base); err != nil {
		return base, fmt.Errorf("parse theme: %w", err)
	}
	return base, nil
}

// Path returns the resolved path to the active theme file.
func Path() (string, error) {
	return themePath()
}

func themePath() (string, error) {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfgHome = filepath.Join(home, ".config")
	}
	return filepath.Join(cfgHome, "gorae", "theme.toml"), nil
}

func (t Theme) IconSet() IconSet {
	mode := strings.ToLower(strings.TrimSpace(t.Icons.Mode))
	var base IconSet
	switch mode {
	case "nerd":
		base = IconSet{
			Favorite:  "",
			ToRead:    "",
			Read:      "",
			Reading:   "",
			Unread:    "○",
			Folder:    "",
			PDF:       "",
			Selected:  "✔",
			Selection: "▌",
		}
	case "ascii":
		base = IconSet{
			Favorite:  "*",
			ToRead:    "t",
			Read:      "v",
			Reading:   ">",
			Unread:    "o",
			Folder:    "[D]",
			PDF:       "[F]",
			Selected:  "*",
			Selection: "|",
		}
	case "off":
		base = IconSet{}
	default:
		// unicode default
		base = IconSet{
			Favorite:  "★",
			ToRead:    "•",
			Read:      "✓",
			Reading:   "▶",
			Unread:    "○",
			Folder:    "▸",
			PDF:       "▣",
			Selected:  "✔",
			Selection: "▌",
		}
	}
	if t.Icons.Favorite != "" {
		base.Favorite = t.Icons.Favorite
	}
	if t.Icons.ToRead != "" {
		base.ToRead = t.Icons.ToRead
	}
	if t.Icons.Read != "" {
		base.Read = t.Icons.Read
	}
	if t.Icons.Reading != "" {
		base.Reading = t.Icons.Reading
	}
	if t.Icons.Unread != "" {
		base.Unread = t.Icons.Unread
	}
	if t.Icons.Folder != "" {
		base.Folder = t.Icons.Folder
	}
	if t.Icons.PDF != "" {
		base.PDF = t.Icons.PDF
	}
	if t.Icons.Selected != "" {
		base.Selected = t.Icons.Selected
	}
	if t.Icons.Selection != "" {
		base.Selection = t.Icons.Selection
	}
	return base
}

func ensureDefaultTheme(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	data := []byte(defaultThemeFile + "\n")
	return os.WriteFile(path, data, 0o644)
}
