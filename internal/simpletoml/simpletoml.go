package simpletoml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Decode parses a limited subset of TOML into the provided structure.
// It supports tables, dotted tables, string/int/bool values, and inline comments.
// The parser converts the TOML document into a map and then leverages the stdlib
// JSON decoder to populate the target structure.
func Decode(data []byte, out interface{}) error {
	tree, err := parseDocument(string(data))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func parseDocument(input string) (map[string]interface{}, error) {
	root := make(map[string]interface{})
	path := []string{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if lineNum == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[[") {
			return nil, fmt.Errorf("arrays of tables not supported (line %d)", lineNum)
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, fmt.Errorf("unterminated table header on line %d", lineNum)
			}
			section := strings.TrimSpace(line[1 : len(line)-1])
			if section == "" {
				return nil, fmt.Errorf("empty table name on line %d", lineNum)
			}
			path = strings.Split(section, ".")
			for i := range path {
				path[i] = strings.TrimSpace(path[i])
			}
			continue
		}
		line = stripInlineComment(line)
		if line == "" {
			continue
		}
		eq := strings.IndexRune(line, '=')
		if eq == -1 {
			return nil, fmt.Errorf("invalid assignment on line %d", lineNum)
		}
		key := strings.TrimSpace(line[:eq])
		valuePart := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf("empty key on line %d", lineNum)
		}
		val, err := parseValue(valuePart)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		if err := insertValue(root, path, key, val); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return root, nil
}

func stripInlineComment(line string) string {
	var b strings.Builder
	inQuote := rune(0)
	escape := false
	for _, r := range line {
		if escape {
			b.WriteRune(r)
			escape = false
			continue
		}
		if inQuote != 0 {
			if r == '\\' && inQuote == '"' {
				escape = true
				b.WriteRune(r)
				continue
			}
			if r == inQuote {
				inQuote = 0
			}
			b.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			b.WriteRune(r)
			continue
		}
		if r == '#' || r == ';' {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func insertValue(root map[string]interface{}, path []string, key string, val interface{}) error {
	target := root
	for _, segment := range path {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		next, exists := target[segment]
		if !exists {
			child := make(map[string]interface{})
			target[segment] = child
			target = child
			continue
		}
		child, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("cannot redefine non-table %s", strings.Join(path, "."))
		}
		target = child
	}
	if _, exists := target[key]; exists {
		return fmt.Errorf("duplicate key %s in section %s", key, strings.Join(path, "."))
	}
	target[key] = val
	return nil
}

func parseValue(raw string) (interface{}, error) {
	if raw == "" {
		return "", nil
	}
	if raw[0] == '"' {
		if !strings.HasSuffix(raw, "\"") || len(raw) == 1 {
			return nil, fmt.Errorf("unterminated string value")
		}
		str, err := strconv.Unquote(raw)
		if err != nil {
			return nil, err
		}
		return str, nil
	}
	if raw[0] == '\'' {
		if !strings.HasSuffix(raw, "'") || len(raw) == 1 {
			return nil, fmt.Errorf("unterminated literal string")
		}
		return raw[1 : len(raw)-1], nil
	}
	lower := strings.ToLower(raw)
	if lower == "true" {
		return true, nil
	}
	if lower == "false" {
		return false, nil
	}
	if strings.ContainsAny(raw, ".eE") {
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f, nil
		}
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i, nil
	}
	if _, err := strconv.ParseUint(strings.TrimPrefix(raw, "+"), 10, 64); err == nil {
		if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return i, nil
		}
	}
	// bare words: treat as strings
	if utf8.ValidString(raw) {
		return raw, nil
	}
	return nil, fmt.Errorf("unsupported value %q", raw)
}
