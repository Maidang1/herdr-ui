//go:build !windows

package localipc

import (
	"context"
	"net"
)

func DialContext(ctx context.Context, path string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}

// Listen creates a local endpoint using the same transport as DialContext.
func Listen(path string) (net.Listener, error) { return net.Listen("unix", path) }
