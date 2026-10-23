package herdr

import (
	"reflect"
	"testing"
)

func TestResolveTarget(t *testing.T) {
	for _, tt := range []struct {
		name, explicit string
		env            map[string]string
		want           string
		bad            bool
	}{
		{name: "default", want: "/home/test/.config/herdr/herdr.sock"},
		{name: "named", env: map[string]string{"HERDR_SESSION": "demo"}, want: "/home/test/.config/herdr/sessions/demo/herdr.sock"},
		{name: "dotted named", env: map[string]string{"HERDR_SESSION": ".demo-1"}, want: "/home/test/.config/herdr/sessions/.demo-1/herdr.sock"},
		{name: "xdg", env: map[string]string{"XDG_CONFIG_HOME": "/cfg"}, want: "/cfg/herdr/herdr.sock"},
		{name: "config does not change socket", env: map[string]string{"HERDR_CONFIG_PATH": "/custom/config.toml"}, want: "/home/test/.config/herdr/herdr.sock"},
		{name: "override", explicit: "/explicit.sock", env: map[string]string{"HERDR_SOCKET_PATH": "/env.sock", "HERDR_SESSION": "demo"}, want: "/explicit.sock"},
		{name: "env override", env: map[string]string{"HERDR_SOCKET_PATH": "/env.sock", "HERDR_SESSION": "../bad"}, want: "/env.sock"},
		{name: "invalid session", env: map[string]string{"HERDR_SESSION": "../bad"}, bad: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTarget(tt.explicit, "/home/test", func(k string) string { return tt.env[k] })
			if (err != nil) != tt.bad {
				t.Fatalf("error = %v", err)
			}
			if !tt.bad && got.Socket != tt.want {
				t.Fatalf("socket = %q, want %q", got.Socket, tt.want)
			}
		})
	}
}

func TestAttachCommandUsesResolvedTarget(t *testing.T) {
	target := Target{Socket: "/tmp/a space.sock"}
	args, env, err := target.Attach("/bin/herdr", "term_1", []string{"A=B", "HERDR_SOCKET_PATH=old", "HERDR_SESSION=other", "HERDR_PANE_ID=wrong", "HERDR_WORKSPACE_ID=wrong", "HERDR_TAB_ID=wrong"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"/bin/herdr", "terminal", "attach", "term_1"}) {
		t.Fatal(args)
	}
	if !reflect.DeepEqual(env, []string{"A=B", "HERDR_SOCKET_PATH=/tmp/a space.sock"}) {
		t.Fatal(env)
	}
	if _, _, err := target.Attach("herdr", "", nil); err == nil {
		t.Fatal("empty terminal accepted")
	}
}
