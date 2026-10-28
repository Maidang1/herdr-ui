package localipc

import "testing"

func TestWindowsPipeNameMatchesInterprocessGenericNamespaced(t *testing.T) {
	for _, tt := range []struct{ path, want string }{
		{`C:\Users\A B\AppData\Roaming\herdr\herdr.sock`, `\\.\pipe\C:\Users\A B\AppData\Roaming\herdr\herdr.sock`},
		{`C:/Users/用户/herdr.sock`, `\\.\pipe\C:/Users/用户/herdr.sock`},
		{`relative.sock`, `\\.\pipe\relative.sock`},
		{`\\.\pipe\already`, `\\.\pipe\\\.\pipe\already`},
	} {
		// Even an apparent pipe name is a namespaced string upstream: no
		// cleaning, separator conversion, or special casing is permitted.
		if got := WindowsPipeName(tt.path); got != tt.want {
			t.Fatalf("WindowsPipeName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
