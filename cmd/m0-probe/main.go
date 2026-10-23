// m0-probe inspects the target without taking ownership of a terminal.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/input"
	"github.com/bytedance/herdr-ui/internal/layout"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	socket := flag.String("socket", "", "Herdr API socket override")
	binary := flag.String("herdr", "herdr", "Herdr CLI executable")
	config := flag.String("config", "", "Herdr config file override (bindings only)")
	flag.Parse()
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	target, err := herdr.ResolveTarget(*socket, home, os.Getenv)
	if err != nil {
		return err
	}
	configPath := *config
	if configPath == "" {
		configPath = os.Getenv("HERDR_CONFIG_PATH")
	}
	if configPath == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		configPath = filepath.Join(base, "herdr", "config.toml")
	}
	data, err := os.ReadFile(configPath)
	source := configPath
	if os.IsNotExist(err) {
		data = nil
		source = "verified defaults (no config file)"
	} else if err != nil {
		return err
	}
	bindings, err := input.Resolve(data, source)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	version, err := exec.CommandContext(ctx, *binary, "--version").Output()
	if err != nil {
		return err
	}
	schema, err := exec.CommandContext(ctx, *binary, "api", "schema", "--json").Output()
	if err != nil {
		return err
	}
	var ping json.RawMessage
	if err := herdr.Call(ctx, target, "ping", nil, &ping); err != nil {
		return err
	}
	var result struct {
		Snapshot struct {
			Version  string `json:"version"`
			Protocol int    `json:"protocol"`
			Panes    []struct {
				PaneID     string `json:"pane_id"`
				TerminalID string `json:"terminal_id"`
				TabID      string `json:"tab_id"`
			} `json:"panes"`
			Tabs []struct {
				TabID string `json:"tab_id"`
			} `json:"tabs"`
		} `json:"snapshot"`
	}
	if err := herdr.Call(ctx, target, "session.snapshot", nil, &result); err != nil {
		return err
	}
	type treeReport struct {
		TabID string   `json:"tab_id"`
		Paths [][]bool `json:"split_paths"`
	}
	trees := []treeReport{}
	for _, tab := range result.Snapshot.Tabs {
		var exported struct {
			Layout struct {
				Root layout.Node `json:"root"`
			} `json:"layout"`
		}
		if err := herdr.Call(ctx, target, "layout.export", map[string]string{"tab_id": tab.TabID}, &exported); err != nil {
			return err
		}
		paths, err := layout.SplitPaths(&exported.Layout.Root)
		if err != nil {
			return fmt.Errorf("tab %s layout: %w", tab.TabID, err)
		}
		trees = append(trees, treeReport{tab.TabID, paths})
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Go           string          `json:"go"`
		Architecture string          `json:"architecture"`
		Socket       string          `json:"socket"`
		CLI          string          `json:"cli"`
		SchemaSHA256 string          `json:"schema_sha256"`
		Ping         json.RawMessage `json:"ping"`
		Snapshot     any             `json:"snapshot"`
		Bindings     input.Bindings  `json:"bindings"`
		Layouts      []treeReport    `json:"layouts"`
	}{runtime.Version(), runtime.GOOS + "/" + runtime.GOARCH, target.Socket, string(version), fmt.Sprintf("%x", sha256.Sum256(schema)), ping, result.Snapshot, bindings, trees})
}
