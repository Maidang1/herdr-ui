package localipc

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func DialContext(ctx context.Context, path string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, WindowsPipeName(path))
}

// Listen creates a byte-mode pipe restricted to the current user. Addr returns
// the logical socket path so it can be passed directly to DialContext.
func Listen(path string) (net.Listener, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	listener, err := winio.ListenPipe(WindowsPipeName(path), &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")",
	})
	if err != nil {
		return nil, err
	}
	return &logicalListener{Listener: listener, path: path}, nil
}

type logicalListener struct {
	net.Listener
	path string
}

func (l *logicalListener) Addr() net.Addr { return logicalAddress(l.path) }

type logicalAddress string

func (a logicalAddress) Network() string { return "pipe" }
func (a logicalAddress) String() string  { return string(a) }
