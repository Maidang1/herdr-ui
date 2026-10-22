package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The overrides file is written one key at a time, so that the comments,
// the order and the shape of the file stay as the user left them.
//
// The file is a list of lines. A line that opens a table names the table
// the keys after it belong to; a line that holds a key sets it.

// setKey writes one key of a TOML file.
//
// The key is dotted: `terminal.size` is the key `size` of the table
// `terminal`. A key that is already there is written over; a key that is
// not is added to its table, and the table itself is added when it is
// missing.
func setKey(path, key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return fmt.Errorf("herdr-ui: %q is not a setting", key)
	}
	table := ""
	if len(parts) > 1 {
		table = strings.Join(parts[:len(parts)-1], ".")
	}
	name := parts[len(parts)-1]

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	lines, found := writeKey(lines, table, name, value)
	if found {
		return writeLines(path, lines)
	}
	lines = insertKey(lines, table, name, value)
	return writeLines(path, lines)
}

// removeKey takes one key out of a TOML file.
//
// The table the key was in is taken out as well when it holds nothing else,
// so that an unbound action leaves no empty table behind.
func removeKey(path, key string) error {
	parts := strings.Split(key, ".")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return fmt.Errorf("herdr-ui: %q is not a setting", key)
	}
	table := ""
	if len(parts) > 1 {
		table = strings.Join(parts[:len(parts)-1], ".")
	}
	name := parts[len(parts)-1]

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	// The block of the table runs from its header to the next one.
	start, end := -1, len(lines)
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isTableHeader(trimmed) {
			continue
		}
		if start >= 0 {
			end = index
			break
		}
		if tableName(trimmed) == table {
			start = index
		}
	}
	if start < 0 {
		return nil
	}
	block := lines[start:end]
	kept := make([]string, 0, len(block))
	removed := false
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if key, _, ok := splitKey(trimmed); ok && key == name {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return nil
	}
	// The table may hold nothing else now.
	for _, line := range kept {
		if _, _, ok := splitKey(strings.TrimSpace(line)); ok {
			out := append([]string{}, lines[:start]...)
			out = append(out, kept...)
			out = append(out, lines[end:]...)
			return writeLines(path, out)
		}
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, lines[end:]...)
	return writeLines(path, out)
}

// readLines reads the lines of a file, or an empty file when it is not
// there yet.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// writeKey writes a key over the one that is there, and says whether it
// found it.
func writeKey(lines []string, table, name, value string) ([]string, bool) {
	current := ""
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isTableHeader(trimmed) {
			current = tableName(trimmed)
			continue
		}
		if current != table {
			continue
		}
		key, _, ok := splitKey(trimmed)
		if !ok || key != name {
			continue
		}
		lines[index] = name + " = " + value
		return lines, true
	}
	return lines, false
}

// insertKey adds a key to a table, and the table to the file when it is
// not there yet.
func insertKey(lines []string, table, name, value string) []string {
	if table == "" {
		return append(lines, name+" = "+value)
	}
	// The block of the table runs from its header to the next one.
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isTableHeader(trimmed) {
			continue
		}
		if start >= 0 {
			// The table ends before this header.
			return insertAt(lines, start, index, name, value)
		}
		if tableName(trimmed) == table {
			start = index
		}
	}
	if start < 0 {
		// The table is not there yet.
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "["+table+"]")
		return append(lines, name+" = "+value)
	}
	return insertAt(lines, start, len(lines), name, value)
}

// insertAt puts a key at the end of a block of lines.
func insertAt(lines []string, start, end int, name, value string) []string {
	at := end
	for at > start+1 && strings.TrimSpace(lines[at-1]) == "" {
		at--
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, name+" = "+value)
	out = append(out, lines[at:]...)
	return out
}

// isTableHeader says a line opens a table.
func isTableHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}

// tableName says the table a header opens.
func tableName(line string) string {
	return strings.TrimSpace(line[1 : len(line)-1])
}

// splitKey says the name of the key a line holds, and the rest of the line.
func splitKey(line string) (string, string, bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	index := strings.Index(line, "=")
	if index < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:]), true
}

// writeLines writes the lines of a file, the way it is written when the
// window is done with it: whole, and with the rights only the user has.
func writeLines(path string, lines []string) error {
	folder := filepath.Dir(path)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	text := ""
	if len(lines) > 0 {
		text = strings.Join(lines, "\n") + "\n"
	}
	temporary, err := os.CreateTemp(folder, ".config-*.toml")
	if err != nil {
		return err
	}
	name := temporary.Name()
	if _, err := temporary.WriteString(text); err != nil {
		temporary.Close()
		os.Remove(name)
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
