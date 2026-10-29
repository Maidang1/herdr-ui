package main

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// Parse Windows argv directly so editor arguments and paths never pass through
// cmd.exe expansion. EDITOR supports a quoted executable and optional arguments.
func editorCommand(path string) ([]string, error) {
	editor := os.Getenv("EDITOR")
	if strings.TrimSpace(editor) == "" {
		return []string{"notepad.exe", path}, nil
	}
	args, err := windows.DecomposeCommandLine(editor)
	if err != nil {
		return nil, err
	}
	return append(args, path), nil
}
