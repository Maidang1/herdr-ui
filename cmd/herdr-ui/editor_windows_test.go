package main

import (
	"reflect"
	"testing"
)

func TestWindowsEditorCommand(t *testing.T) {
	path := `C:\Users\A B\config %PATH% & draft.toml`
	for _, tt := range []struct {
		name, editor string
		want         []string
	}{
		{"default", "", []string{"notepad.exe", path}},
		{"quoted executable and arguments", `"C:\Program Files\Editor\editor.exe" --wait`, []string{`C:\Program Files\Editor\editor.exe`, "--wait", path}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EDITOR", tt.editor)
			got, err := editorCommand(path)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("editor = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
