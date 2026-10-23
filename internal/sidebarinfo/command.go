package sidebarinfo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

const commandLimit = 2 << 20

type commandOutput struct {
	data       []byte
	code       int
	diagnostic bool
}
type commandFunc func(context.Context, string, ...string) (commandOutput, error)

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("sidebar metadata output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func runCommand(ctx context.Context, name string, args ...string) (commandOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Env = gitEnvironment(os.Environ())
	stdout, stderr := &limitedBuffer{limit: commandLimit}, &limitedBuffer{limit: 64 << 10}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
	}
	return commandOutput{data: stdout.Bytes(), code: code, diagnostic: stderr.Len() != 0}, err
}
func gitEnvironment(inherited []string) []string {
	env := make([]string, 0, len(inherited)+2)
	for _, value := range inherited {
		key, _, _ := strings.Cut(value, "=")
		// A terminal's inherited Git variables must not redirect local metadata to
		// the caller repository or inject command/config overrides.
		if strings.HasPrefix(key, "GIT_") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
}
