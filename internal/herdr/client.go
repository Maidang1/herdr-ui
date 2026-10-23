package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const MaxLineBytes = 16 * 1024 * 1024

// Call is a one-shot, bounded M0 probe. M1 will own persistent serial request
// and subscription lanes. Calls are never retried, including mutations.
func Call(ctx context.Context, target Target, method string, params any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", target.Socket)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	if params == nil {
		params = struct{}{}
	}
	request := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{"m0-1", method, params}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return fmt.Errorf("%s write: %w", method, err)
	}
	line, err := readLine(bufio.NewReader(conn))
	if err != nil {
		return fmt.Errorf("%s response: %w", method, err)
	}
	var response struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(line, &response); err != nil {
		return fmt.Errorf("%s JSON: %w", method, err)
	}
	if response.ID != request.ID {
		return fmt.Errorf("%s: response ID mismatch", method)
	}
	if len(response.Error) > 0 && string(response.Error) != "null" {
		return fmt.Errorf("%s rejected: %s", method, response.Error)
	}
	if len(response.Result) == 0 || string(response.Result) == "null" {
		return fmt.Errorf("%s: missing result", method)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(response.Result, out)
}

func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > MaxLineBytes {
			return nil, &ProtocolError{fmt.Sprintf("JSON line exceeds %d bytes", MaxLineBytes)}
		}
		line = append(line, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, err
		}
		return line, nil
	}
}
