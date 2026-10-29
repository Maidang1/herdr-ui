package herdr

import "testing"

func TestResolveWindowsTarget(t *testing.T) {
	for _, tt := range []struct {
		name, explicit string
		env            map[string]string
		want           string
	}{
		{"default", "", map[string]string{"APPDATA": `C:\Users\Test\AppData\Roaming`}, `C:\Users\Test\AppData\Roaming\herdr\herdr.sock`},
		{"profile fallback", "", map[string]string{"USERPROFILE": `C:\Users\Test`}, `C:\Users\Test\AppData\Roaming\herdr\herdr.sock`},
		{"named", "", map[string]string{"APPDATA": `C:\Roaming`, "HERDR_SESSION": "demo"}, `C:\Roaming\herdr\sessions\demo\herdr.sock`},
		{"xdg override", "", map[string]string{"XDG_CONFIG_HOME": `D:\cfg`, "APPDATA": `C:\Roaming`}, `D:\cfg\herdr\herdr.sock`},
		{"explicit wins", `D:\socket marker.sock`, map[string]string{"HERDR_SOCKET_PATH": `C:\other.sock`, "HERDR_SESSION": "../bad"}, `D:\socket marker.sock`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTarget(tt.explicit, `C:\Users\Test`, func(k string) string { return tt.env[k] })
			if err != nil || got.Socket != tt.want {
				t.Fatalf("target = %q, %v; want %q", got.Socket, err, tt.want)
			}
		})
	}
}
