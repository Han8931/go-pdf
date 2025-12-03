package ui

import (
    "strings"
)

func RenderPopup(title, body string) string {
    bodyLines := strings.Split(body, "\n")

    width := 40

    top := "╔" + strings.Repeat("═", width-2) + "╗"
    bottom := "╚" + strings.Repeat("═", width-2) + "╝"

    var b strings.Builder
    b.WriteString(top + "\n")

    // title row
    paddedTitle := title + strings.Repeat(" ", width-len(title)-2)
    b.WriteString("║" + paddedTitle + "║\n")

    // empty spacer row
    b.WriteString("║" + strings.Repeat(" ", width-2) + "║\n")

    // body rows
    for _, line := range bodyLines {
        padded := line + strings.Repeat(" ", width-len(line)-2)
        b.WriteString("║" + padded + "║\n")
    }

    b.WriteString(bottom + "\n")
    return b.String()
}
