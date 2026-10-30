package herdr

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

type RestoreSettings struct {
	ResumeAgents bool
	PaneHistory  bool
}

var restoreConfigMu sync.Mutex

func ReadRestoreSettings(path string) (RestoreSettings, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return RestoreSettings{ResumeAgents: true}, nil
	}
	if err != nil {
		return RestoreSettings{}, err
	}
	return parseRestoreSettings(data)
}
func parseRestoreSettings(data []byte) (RestoreSettings, error) {
	var config struct {
		Session struct {
			Resume *bool `toml:"resume_agents_on_restore"`
		} `toml:"session"`
		Experimental struct {
			History *bool `toml:"pane_history"`
		} `toml:"experimental"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return RestoreSettings{}, err
	}
	v := RestoreSettings{ResumeAgents: true}
	if config.Session.Resume != nil {
		v.ResumeAgents = *config.Session.Resume
	}
	if config.Experimental.History != nil {
		v.PaneHistory = *config.Experimental.History
	}
	return v, nil
}

// patchConfigBool edits the parsed boolean's byte range, preserving surrounding
// text, comments and string contents. Unsupported table forms remain untouched.
func patchConfigBool(text, section, key string, value bool) (string, error) {
	data := []byte(text)
	var parser unstable.Parser
	parser.Reset(data)
	var table []string
	insert := -1
	inSection := false
	var replacement *unstable.Range
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Table && node.Kind != unstable.ArrayTable && node.Kind != unstable.KeyValue {
			continue
		}
		var parts []string
		keys := node.Key()
		first := -1
		for keys.Next() {
			part := keys.Node()
			if first < 0 {
				first = int(part.Raw.Offset)
			}
			parts = append(parts, string(part.Data))
		}
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			if inSection && insert < 0 {
				insert = strings.LastIndex(text[:first], "\n") + 1
			}
			table = parts
			inSection = node.Kind == unstable.Table && len(table) == 1 && table[0] == section
			continue
		}
		path := append(append([]string(nil), table...), parts...)
		if len(path) == 2 && path[0] == section && path[1] == key {
			if node.Value().Kind != unstable.Bool {
				return "", fmt.Errorf("%s.%s must be a boolean", section, key)
			}
			r := parser.Range(node.Value().Data)
			replacement = &r
		}
	}
	if err := parser.Error(); err != nil {
		return "", err
	}
	if replacement != nil {
		start := int(replacement.Offset)
		end := start + int(replacement.Length)
		return text[:start] + fmt.Sprint(value) + text[end:], nil
	}
	assignment := key + " = " + fmt.Sprint(value) + "\n"
	if insert >= 0 {
		return text[:insert] + assignment + text[insert:], nil
	}
	if inSection {
		return strings.TrimSuffix(text, "\n") + "\n" + assignment, nil
	}
	return strings.TrimSuffix(text, "\n") + "\n[" + section + "]\n" + assignment, nil
}
func SaveRestoreSettings(path string, v RestoreSettings) error {
	restoreConfigMu.Lock()
	defer restoreConfigMu.Unlock()
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, e := filepath.EvalSymlinks(path)
		if e != nil {
			return e
		}
		path = resolved
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := parseRestoreSettings(data); err != nil {
		return err
	}
	text, err := patchConfigBool(string(data), "session", "resume_agents_on_restore", v.ResumeAgents)
	if err != nil {
		return err
	}
	text, err = patchConfigBool(text, "experimental", "pane_history", v.PaneHistory)
	if err != nil {
		return err
	}
	parsed, err := parseRestoreSettings([]byte(text))
	if err != nil {
		return err
	}
	if parsed != v {
		return fmt.Errorf("configuration uses a TOML form that requires the configuration editor")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".herdr-restore-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	mode := os.FileMode(0600)
	if info, e := os.Stat(path); e == nil {
		mode = info.Mode().Perm()
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.WriteString(text)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if !bytes.Equal(current, data) {
		return fmt.Errorf("Herdr configuration changed while saving; refresh and try again")
	}
	return os.Rename(f.Name(), path)
}
