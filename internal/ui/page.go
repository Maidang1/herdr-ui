package ui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A browser page is a web view in a tab beside the terminal tabs. A local
// file is served from the folder it sits in, so that the files it refers to
// load beside it.

// Page is one browser page.
type Page struct {
	ID    string
	Title string
	URL   string
	File  string
	Zoom  float64

	view   *mygo.WebView
	server *localServer
}

// pageSeq numbers the pages.
var pageSeq atomic.Uint64

// OpenPage opens a page for a URL or a local file and shows it.
func (a *App) OpenPage(target string) (*Page, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("herdr-ui: no page to open")
	}

	url := target
	file := ""
	var server *localServer

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		path, err := filepath.Abs(target)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("herdr-ui: %w", err)
		}
		if info.IsDir() {
			path = filepath.Join(path, "index.html")
			if _, err := os.Stat(path); err != nil {
				return nil, fmt.Errorf("herdr-ui: %s holds no index.html", target)
			}
		}
		server, err = serveFolder(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		file = path
		url = fmt.Sprintf("http://%s/%s", server.address(), filepath.Base(path))
	}

	page := &Page{
		ID:    fmt.Sprintf("page-%d", pageSeq.Add(1)),
		Title: pageTitle(target),
		URL:   url,
		File:  file,
		Zoom:  1,
	}
	if err := a.withWindow(func(win *mygo.Window) error {
		view, err := win.NewWebView(mygo.WebViewOptions{URL: url})
		if err != nil {
			return err
		}
		page.view = view
		view.Page().OnDidNavigate(func(current string) {
			a.mu.Lock()
			page.URL = current
			a.mu.Unlock()
			a.invalidate()
		})
		return nil
	}); err != nil {
		if server != nil {
			server.close()
		}
		return nil, err
	}

	a.mu.Lock()
	a.pages = append(a.pages, page)
	a.page = page.ID
	a.mu.Unlock()
	a.invalidate()
	return page, nil
}

// pageTitle names a page after what it shows.
func pageTitle(target string) string {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(target, "https://"), "http://")
		if index := strings.IndexAny(trimmed, "/?"); index > 0 {
			trimmed = trimmed[:index]
		}
		if trimmed == "" {
			return target
		}
		return trimmed
	}
	return filepath.Base(target)
}

// withWindow runs something on the window's thread.
func (a *App) withWindow(fn func(win *mygo.Window) error) error {
	if a.win == nil {
		return fmt.Errorf("herdr-ui: the window is not open yet")
	}
	result := make(chan error, 1)
	a.win.Update(func() {
		result <- fn(a.win)
	})
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("herdr-ui: the window did not answer")
	}
}

// selectPage shows a page.
func (a *App) selectPage(id string) {
	a.mu.Lock()
	a.page = id
	a.mu.Unlock()
	a.invalidate()
}

// closePage closes a page.
func (a *App) closePage(id string) {
	a.mu.Lock()
	var closed *Page
	var pages []*Page
	for _, page := range a.pages {
		if page.ID == id {
			closed = page
			continue
		}
		pages = append(pages, page)
	}
	a.pages = pages
	if a.page == id {
		a.page = ""
	}
	a.mu.Unlock()
	if closed != nil && closed.server != nil {
		closed.server.close()
	}
	a.invalidate()
}

// Pages returns the pages the window shows.
func (a *App) Pages() []Page {
	a.mu.Lock()
	defer a.mu.Unlock()
	pages := make([]Page, 0, len(a.pages))
	for _, page := range a.pages {
		pages = append(pages, Page{
			ID:    page.ID,
			Title: page.Title,
			URL:   page.URL,
			File:  page.File,
			Zoom:  page.Zoom,
		})
	}
	return pages
}

// Page returns one page by its ID.
func (a *App) Page(id string) (Page, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, page := range a.pages {
		if page.ID == id {
			return Page{ID: page.ID, Title: page.Title, URL: page.URL, File: page.File, Zoom: page.Zoom}, true
		}
	}
	return Page{}, false
}

// ReloadPage reloads a page.
func (a *App) ReloadPage(id string) error {
	page, ok := a.pageByID(id)
	if !ok {
		return fmt.Errorf("herdr-ui: no page %s", id)
	}
	if page.view == nil {
		return fmt.Errorf("herdr-ui: the page %s has no view", id)
	}
	page.view.Page().Reload()
	return nil
}

// NavigatePage loads another address in a page.
func (a *App) NavigatePage(id, url string) error {
	page, ok := a.pageByID(id)
	if !ok {
		return fmt.Errorf("herdr-ui: no page %s", id)
	}
	if page.view == nil {
		return fmt.Errorf("herdr-ui: the page %s has no view", id)
	}
	return page.view.Page().LoadURL(url)
}

// PageBack goes back in a page.
func (a *App) PageBack(id string) error {
	page, ok := a.pageByID(id)
	if !ok {
		return fmt.Errorf("herdr-ui: no page %s", id)
	}
	if page.view == nil {
		return fmt.Errorf("herdr-ui: the page %s has no view", id)
	}
	page.view.Page().GoBack()
	return nil
}

// PageForward goes forward in a page.
func (a *App) PageForward(id string) error {
	page, ok := a.pageByID(id)
	if !ok {
		return fmt.Errorf("herdr-ui: no page %s", id)
	}
	if page.view == nil {
		return fmt.Errorf("herdr-ui: the page %s has no view", id)
	}
	page.view.Page().GoForward()
	return nil
}

// PageText reads the text of a page.
func (a *App) PageText(id string) (string, error) {
	page, ok := a.pageByID(id)
	if !ok {
		return "", fmt.Errorf("herdr-ui: no page %s", id)
	}
	if page.view == nil {
		return "", fmt.Errorf("herdr-ui: the page %s has no view", id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	value, err := page.view.Page().EvalContext(ctx, "document.body ? document.body.innerText : ''")
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Sprint(value), nil
	}
	return text, nil
}

// PageZoom changes the zoom of a page, in percent or as a step.
func (a *App) PageZoom(id, change string) (float64, error) {
	page, ok := a.pageByID(id)
	if !ok {
		return 0, fmt.Errorf("herdr-ui: no page %s", id)
	}
	zoom := page.Zoom
	if zoom == 0 {
		zoom = 1
	}
	switch strings.ToLower(strings.TrimSpace(change)) {
	case "", "reset":
		zoom = 1
	case "+", "in", "up":
		zoom += 0.1
	case "-", "out", "down":
		zoom -= 0.1
	default:
		value, err := strconv.ParseFloat(strings.TrimSuffix(change, "%"), 64)
		if err != nil {
			return 0, fmt.Errorf("herdr-ui: %q is not a zoom", change)
		}
		zoom = value / 100
	}
	if zoom < 0.25 {
		zoom = 0.25
	}
	if zoom > 4 {
		zoom = 4
	}
	a.mu.Lock()
	page.Zoom = zoom
	a.mu.Unlock()
	if page.view != nil {
		page.view.Page().SetZoomFactor(zoom)
	}
	a.invalidate()
	return zoom, nil
}

func (a *App) pageByID(id string) (*Page, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, page := range a.pages {
		if page.ID == id {
			return page, true
		}
	}
	return nil, false
}

// drawPage shows a browser page and the controls that go with it.
func (a *App) drawPage(c *ui.Context, pages []*Page, id string) {
	theme := c.Theme()
	var page *Page
	for _, item := range pages {
		if item.ID == id {
			page = item
		}
	}
	if page == nil {
		ui.Text(c, "The page is gone.").TextColor(theme.TextMuted)
		return
	}
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		ui.Row(c).Padding(6, 10).Gap(6).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "‹").Label("Back").Clicked() {
				_ = a.PageBack(page.ID)
			}
			if ui.Button(c, "›").Label("Forward").Clicked() {
				_ = a.PageForward(page.ID)
			}
			if ui.Button(c, "Reload").Clicked() {
				_ = a.ReloadPage(page.ID)
			}
			zoom := fmt.Sprintf("%d%%", int(page.Zoom*100+0.5))
			if ui.Button(c, zoom).Label("Zoom").Clicked() {
				_, _ = a.PageZoom(page.ID, "+")
			}
			ui.Text(c, page.Title).TextColor(theme.TextMuted)
			ui.Row(c).Grow(1)
			ui.Text(c, page.URL).FontSize(11).TextColor(theme.TextMuted)
		})
		if page.view != nil {
			ui.WebView(c, page.view).Grow(1)
		}
	})
}

// localServer serves the folder a local file sits in.
type localServer struct {
	dir      string
	listener net.Listener
	server   *http.Server
}

// serveFolder starts a server for one folder.
func serveFolder(dir string) (*localServer, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	local := &localServer{dir: absolute, listener: listener}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(absolute)))
	local.server = &http.Server{Handler: local.guard(mux)}
	go func() {
		_ = local.server.Serve(listener)
	}()
	return local, nil
}

// address returns the address the server listens on.
func (s *localServer) address() string {
	return s.listener.Addr().String()
}

// close stops the server.
func (s *localServer) close() {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

// guard keeps the server to the folder it serves: no hidden files, and
// nothing outside.
func (s *localServer) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean("/" + r.URL.Path)
		for _, part := range strings.Split(clean, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		target := filepath.Join(s.dir, filepath.FromSlash(clean))
		if !strings.HasPrefix(target, s.dir) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
