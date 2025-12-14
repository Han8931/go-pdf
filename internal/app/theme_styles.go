package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gorae/internal/theme"
)

type panelStyles struct {
	Header         lipgloss.Style
	Body           lipgloss.Style
	Info           lipgloss.Style
	Active         lipgloss.Style
	Selected       lipgloss.Style
	Cursor         lipgloss.Style
	CursorSelected lipgloss.Style
}

type viewStyles struct {
	AppHeader   lipgloss.Style
	Tree        panelStyles
	List        panelStyles
	Preview     panelStyles
	StatusBar   lipgloss.Style
	StatusLabel lipgloss.Style
	StatusValue lipgloss.Style
	PromptLabel lipgloss.Style
	PromptValue lipgloss.Style
	Separator   lipgloss.Style
	MetaOverlay lipgloss.Style
	Border      lipgloss.Style
	SepChar     string
}

type borderCharset struct {
	Vertical    string
	Horizontal  string
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
}

func newViewStyles(th theme.Theme) viewStyles {
	return viewStyles{
		AppHeader: styleFromSpec(th.Components.AppHeader),
		Tree: panelStyles{
			Header: styleFromSpec(th.Components.TreeHeader),
			Body:   styleFromSpec(th.Components.TreeBody),
			Info:   styleFromSpec(th.Components.TreeInfo),
			Active: styleFromSpec(th.Components.TreeActive),
		},
		List: panelStyles{
			Header:         styleFromSpec(th.Components.ListHeader),
			Body:           styleFromSpec(th.Components.ListBody),
			Selected:       styleFromSpec(th.Components.ListSelected),
			Cursor:         styleFromSpec(th.Components.ListCursor),
			CursorSelected: styleFromSpec(th.Components.ListCursorSelect),
		},
		Preview: panelStyles{
			Header: styleFromSpec(th.Components.PreviewHeader),
			Body:   styleFromSpec(th.Components.PreviewBody),
		},
		StatusBar:   styleFromSpec(th.Components.StatusBar),
		StatusLabel: styleFromSpec(th.Components.StatusLabel),
		StatusValue: styleFromSpec(th.Components.StatusValue),
		PromptLabel: styleFromSpec(th.Components.PromptLabel),
		PromptValue: styleFromSpec(th.Components.PromptValue),
		Separator:   styleFromSpec(th.Components.Separator),
		MetaOverlay: styleFromSpec(th.Components.MetaOverlay),
		Border:      lipgloss.NewStyle().Foreground(lipgloss.Color(strings.TrimSpace(th.Borders.Color))),
		SepChar:     borderCharsetFor(th.Borders.Style).Vertical,
	}
}

func styleFromSpec(spec theme.StyleSpec) lipgloss.Style {
	style := lipgloss.NewStyle()
	if fg := strings.TrimSpace(spec.FG); fg != "" {
		style = style.Foreground(lipgloss.Color(fg))
	}
	if bg := strings.TrimSpace(spec.BG); bg != "" {
		style = style.Background(lipgloss.Color(bg))
	}
	if spec.Bold {
		style = style.Bold(true)
	}
	if spec.Italic {
		style = style.Italic(true)
	}
	if spec.Faint {
		style = style.Faint(true)
	}
	return style
}

func borderCharsetFor(style string) borderCharset {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "rounded":
		return borderCharset{
			Vertical:    "│",
			Horizontal:  "─",
			TopLeft:     "╭",
			TopRight:    "╮",
			BottomLeft:  "╰",
			BottomRight: "╯",
		}
	case "thick":
		return borderCharset{
			Vertical:    "┃",
			Horizontal:  "━",
			TopLeft:     "┏",
			TopRight:    "┓",
			BottomLeft:  "┗",
			BottomRight: "┛",
		}
	case "none":
		return borderCharset{
			Vertical:    " ",
			Horizontal:  " ",
			TopLeft:     " ",
			TopRight:    " ",
			BottomLeft:  " ",
			BottomRight: " ",
		}
	default:
		return borderCharset{
			Vertical:    "┃",
			Horizontal:  "─",
			TopLeft:     "┌",
			TopRight:    "┐",
			BottomLeft:  "└",
			BottomRight: "┘",
		}
	}
}
