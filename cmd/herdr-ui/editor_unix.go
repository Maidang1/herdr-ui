//go:build !windows

package main

func editorCommand(path string) ([]string, error) {
	return []string{"/bin/sh", "-c", `exec ${EDITOR:-vi} "$1"`, "herdr-editor", path}, nil
}
