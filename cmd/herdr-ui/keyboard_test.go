package main

import (
	"github.com/bytedance/herdr-ui/internal/endpoint"
	"github.com/bytedance/herdr-ui/internal/input"
	"testing"
)

func TestCommandCatalogMatchesOpaqueIDAndAction(t *testing.T) {
	bindings, err := input.Resolve([]byte("[[keys.command]]\nkey='prefix+y'\ncommand='echo hello'\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	state := endpoint.State{Connected: true, Snapshot: &endpoint.Snapshot{Commands: []endpoint.Command{{CommandID: "opaque-new-boot", BindingLabels: []string{"prefix+y"}, Action: "Shell"}}}}
	id, err := matchCommand(state, bindings, 0)
	if err != nil || id != "opaque-new-boot" {
		t.Fatal(id, err)
	}
	state.Snapshot.Commands[0].Action = "Popup"
	if _, err := matchCommand(state, bindings, 0); err == nil {
		t.Fatal("wrong command kind accepted")
	}
	state.Snapshot.Commands[0].Action = "Shell"
	state.Snapshot.Commands = append(state.Snapshot.Commands, endpoint.Command{CommandID: "ambiguous", BindingLabels: []string{"prefix+y"}, Action: "Shell"})
	if _, err := matchCommand(state, bindings, 0); err == nil {
		t.Fatal("ambiguous command accepted")
	}
}
