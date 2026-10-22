// Command fakessh stands in for the ssh binary in the tests of the bridge.
//
// It offers the status of a copy of herdr, waits for the answer, and then
// speaks the endpoint protocol over the connection, the way the remote
// host does.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// report writes what went wrong, to a file when the test asks for one.
func report(message string, args ...any) {
	text := fmt.Sprintf(message, args...)
	if path := os.Getenv("FAKESSH_ERR"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err == nil {
			fmt.Fprintln(file, text)
			file.Close()
			return
		}
	}
	fmt.Fprintln(os.Stderr, text)
}

func main() {
	offer := os.Getenv("FAKESSH_OFFER")
	if offer == "" {
		offer = "{}"
	}
	welcome, err := os.ReadFile(os.Getenv("FAKESSH_WELCOME"))
	if err != nil {
		report("fakessh:", err)
		os.Exit(1)
	}

	fmt.Printf("%s\n", offer)
	fmt.Printf("\n%s\n", "herdr-remote-output-ready:1")

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		report("fakessh:", err)
		os.Exit(1)
	}
	if answer != "accept\n" && answer != "accept-idle\n" {
		report("fakessh: the answer was %q", answer)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(welcome); err != nil {
		os.Exit(1)
	}
	// The frames of the protocol, and the answers of the daemon.
	for {
		var header [4]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return
		}
		payload := make([]byte, binary.LittleEndian.Uint32(header[:]))
		if _, err := io.ReadFull(reader, payload); err != nil {
			return
		}
	}
}
