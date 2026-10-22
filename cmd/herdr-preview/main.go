// Command herdr-preview draws the window without opening it, and writes
// the pictures of it.
//
// It is how the window is checked against the design: the pictures it
// writes are the ones to look at beside the boards of docs.pen/01.pen.
//
//	go run ./cmd/herdr-preview -out /tmp/herdr
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	herdr "herdr-ui/internal/ui"

	"github.com/egoist/mygo/ui"
)

func main() {
	out := flag.String("out", ".", "the directory the pictures are written to")
	width := flag.Int("width", 1440, "the width of the window")
	height := flag.Int("height", 900, "the height of the window")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-preview:", err)
		os.Exit(1)
	}
	for _, mode := range []struct {
		name string
		mode herdr.Mode
	}{
		{"light", herdr.ModeLight},
		{"dark", herdr.ModeDark},
	} {
		app := herdr.New()
		app.SetMode(mode.mode)
		tester := ui.NewTester(app.View, *width, *height)
		tester.Frame()
		path := filepath.Join(*out, "herdr-"+mode.name+".png")
		file, err := os.Create(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "herdr-preview:", err)
			os.Exit(1)
		}
		if err := png.Encode(file, tester.Image()); err != nil {
			fmt.Fprintln(os.Stderr, "herdr-preview:", err)
			os.Exit(1)
		}
		file.Close()
		fmt.Println(path)
	}
}
