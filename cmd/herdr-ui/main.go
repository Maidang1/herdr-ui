// Command herdr-ui is the window of the Herdr client.
//
// The window shows the session the daemon draws: the terminals of the
// panels, the workspaces, the tabs, and the agents that work in them.
//
//	go run ./cmd/herdr-ui
package main

import (
	"log"

	herdr "herdr-ui/internal/ui"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	app := herdr.New()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:         "Herdr",
			Width:         1440,
			Height:        900,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(app.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
