package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// configWatcher watches the config file and, when it changes, fully reloads
// the configuration and re-resolves the active window against the fresh config.
// It runs as its own goroutine (see NewConfigWatcher) so a running shuttle-go
// picks up edits to the config file without a restart.
//
// It uses fsnotify to watch the config file's parent directory (so both
// in-place saves and temp-file+rename saves are caught) and falls back to
// polling on a 5 second interval if the watcher cannot be set up.
type configWatcher struct {
	path     string
	watcher  *watcher
	lastMod  time.Time
	lastSize int64
}

// NewConfigWatcher builds a configWatcher for the given config file path.
// watcher is the window watcher used to re-resolve the active app after a
// reload; it may be nil (in which case a reload updates loadedConfiguration but
// leaves currentConfiguration as-is).
func NewConfigWatcher(path string, watcher *watcher) *configWatcher {
	cw := &configWatcher{path: path, watcher: watcher}
	if st, err := os.Stat(path); err == nil {
		cw.lastMod = st.ModTime()
		cw.lastSize = st.Size()
	}
	return cw
}

// Run blocks, watching the config file and reloading it on change. It uses
// fsnotify when available and otherwise polls every 5 seconds.
func (cw *configWatcher) Run() {
	w, err := fsnotify.NewWatcher()
	if err != nil || w == nil {
		cw.runPolling(5 * time.Second)
		return
	}
	defer w.Close()

	// Watch the parent directory rather than the file itself: editors commonly
	// save by writing a temp file and renaming it over the original, which
	// replaces the file's inode and would orphan a watch on the file. Watching
	// the directory catches the create/rename of the filename either way.
	dir := filepath.Dir(cw.path)
	name := filepath.Base(cw.path)
	if err := w.Add(dir); err != nil {
		cw.runPolling(5 * time.Second)
		return
	}

	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			// Only react to events for our config file.
			if filepath.Base(event.Name) != name {
				continue
			}
			cw.check()
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
			// The watcher is gone; fall back to polling.
			cw.runPolling(5 * time.Second)
			return
		}
	}
}

// runPolling blocks, re-checking the file every interval.
func (cw *configWatcher) runPolling(interval time.Duration) {
	for {
		cw.check()
		time.Sleep(interval)
	}
}

// check reloads the config if the file's mtime or size changed since the last
// check. A failed reload (e.g. a transiently invalid edit) is reported but not
// fatal: the previous config stays in effect and the next tick retries.
func (cw *configWatcher) check() {
	st, err := os.Stat(cw.path)
	if err != nil {
		return
	}
	if st.ModTime().Equal(cw.lastMod) && st.Size() == cw.lastSize {
		return
	}

	if *debugMode {
		fmt.Println("Config file changed, reloading:", cw.path)
	}
	if err := LoadConfig(cw.path); err != nil {
		fmt.Println("Error reloading configuration (keeping previous):", err)
		return
	}

	cw.lastMod = st.ModTime()
	cw.lastSize = st.Size()

	if *debugMode {
		fmt.Println("Configuration reloaded")
	}
	if cw.watcher != nil {
		cw.watcher.reapplyWindow()
	}
}
