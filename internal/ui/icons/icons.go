// The icons of the window.
//
// They are the icons of the design file, which are the icons of Lucide, and
// the mark of GitHub. The files are read once, when the window first asks
// for one of them.
//
// The licenses of the icons are in NOTICE.md beside them.
package icons

import (
	"embed"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed *.svg
var files embed.FS

var (
	once  sync.Once
	icons map[string]*ui.SVG
)

// Icon returns the icon of a name, as the design file names it.
//
// The name is the name of the file without its suffix: "panel-left",
// "git-branch". An icon that is not there is nothing, so that a window
// that asks for one does not stop.
func Icon(name string) *ui.SVG {
	once.Do(func() {
		icons = map[string]*ui.SVG{}
		entries, err := files.ReadDir(".")
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".svg") {
				continue
			}
			data, err := files.ReadFile(entry.Name())
			if err != nil {
				continue
			}
			parsed, err := ui.ParseSVG(data)
			if err != nil {
				continue
			}
			icons[strings.TrimSuffix(entry.Name(), ".svg")] = parsed
		}
	})
	return icons[name]
}
