package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgb/xtest"
	"github.com/godbus/dbus/v5"
)

type watcher struct {
	conn                      *xgb.Conn
	root                      xproto.Window
	activeAtom, nameAtom      xproto.Atom
	classAtom                 xproto.Atom
	prevWindowName            string
	prevWMClass               string
	lastWindowID              xproto.Window
}

func NewWindowWatcher() *watcher {
	return &watcher{}
}

// dbusSession returns a shared session-bus connection, or nil if the session
// bus is unavailable. It is only dialed on first use, which happens only on
// Wayland; the connection is shared and goroutine-safe, so it is reused across
// the watcher loop and the mouse-peek path.
var dbusSession *dbus.Conn

func sessionBus() *dbus.Conn {
	if dbusSession != nil {
		return dbusSession
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	dbusSession = conn
	return conn
}

func getWaylandWindow() (title, wmClass string) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", ""
	}

	conn := sessionBus()
	if conn == nil {
		return "", ""
	}

	// GNOME Shell "Windows" extension: list all windows as JSON
	var out string
	call := conn.Object("org.gnome.Shell", "/org/gnome/Shell/Extensions/Windows").
		Call("org.gnome.Shell.Extensions.Windows.List", 0)
	if err := call.Store(&out); err != nil || out == "" {
		return "", ""
	}

	type window struct {
		Title   string `json:"title"`
		WMClass string `json:"wm_class"`
		Focus   bool   `json:"focus"`
	}
	var windows []window
	if json.Unmarshal([]byte(out), &windows) == nil {
		for _, w := range windows {
			if w.Focus {
				return w.Title, w.WMClass
			}
		}
	}

	return "", ""
}

// waylandInfo is the single object the Shuttle Pro GNOME extension returns
// from its Info method: the focused window's title and wm_class, plus the
// current pointer position.
type waylandInfo struct {
	Title   *string `json:"title"`
	WMClass *string `json:"wm_class"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
}

// getWaylandInfo queries the Shuttle Pro GNOME extension over D-Bus for the
// current shell state (focused window + pointer position). That extension reads
// the real values from the shell, which /dev/input/mice and the X11 window
// properties cannot on Wayland. It returns ok=false when not on Wayland or the
// extension is not installed/enabled; the window fields are then left unset
// (the extension reports them as null when no window is focused).
func getWaylandInfo() (info *waylandInfo, ok bool) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, false
	}

	conn := sessionBus()
	if conn == nil {
		return nil, false
	}

	var out string
	call := conn.Object("org.gnome.Shell", "/org/gnome/Shell/Extensions/ShuttlePro").
		Call("org.gnome.Shell.Extensions.ShuttlePro.Info", 0)
	if err := call.Store(&out); err != nil || out == "" {
		return nil, false
	}

	// The extension returns the shell state as a single JSON object.
	var w waylandInfo
	if json.Unmarshal([]byte(out), &w) == nil {
		return &w, true
	}
	return nil, false
}

func (w *watcher) Setup() error {
	if isWayland() {
		// No X server on Wayland; window-title matching is driven by
		// getWaylandWindowTitle() in watch().
		return nil
	}

	X, err := xgb.NewConn()
	if err != nil {
		return err
	}

	// Get the window id of the root window.
	setup := xproto.Setup(X)

	if err := xtest.Init(X); err != nil {
		return err
	}

	w.conn = X
	w.root = setup.DefaultScreen(X).Root

	// Get the atom id (i.e., intern an atom) of "_NET_ACTIVE_WINDOW".
	aname := "_NET_ACTIVE_WINDOW"
	activeAtom, err := xproto.InternAtom(X, true, uint16(len(aname)),
		aname).Reply()
	if err != nil {
		return fmt.Errorf("Couldn't get _NET_ACTIVE_WINDOW atom: %s", err)
	}

	// Get the atom id (i.e., intern an atom) of "_NET_WM_NAME".
	aname = "_NET_WM_NAME"
	nameAtom, err := xproto.InternAtom(X, true, uint16(len(aname)),
		aname).Reply()
	if err != nil {
		return fmt.Errorf("Couldn't get _NET_WM_NAME atom: %s", err)
	}

	// Get the atom id of "WM_CLASS".
	aname = "WM_CLASS"
	classAtom, err := xproto.InternAtom(X, true, uint16(len(aname)),
		aname).Reply()
	if err != nil {
		return fmt.Errorf("Couldn't get WM_CLASS atom: %s", err)
	}

	w.activeAtom = activeAtom.Atom
	w.nameAtom = nameAtom.Atom
	w.classAtom = classAtom.Atom

	return nil
}

func (w *watcher) Run() {
	for {
		w.watch()
		time.Sleep(2 * time.Second)
	}
}

func (w *watcher) watch() {
	var windowName, wmClass string

	if isWayland() {
		windowName, wmClass = getWaylandWindow()
		if windowName == "" && wmClass == "" {
			return
		}
	} else {
		// From github.com/BurntSushi/xgb's examples.
		reply, err := xproto.GetProperty(w.conn, false, w.root, w.activeAtom,
			xproto.GetPropertyTypeAny, 0, (1<<32)-1).Reply()
		if err != nil {
			fmt.Println("watch windows, failed to get window properties:", err)
			return
		}
		windowID := xproto.Window(xgb.Get32(reply.Value))

		reply, err = xproto.GetProperty(w.conn, false, windowID, w.nameAtom,
			xproto.GetPropertyTypeAny, 0, (1<<32)-1).Reply()
		if err != nil {
			fmt.Println("watch windows, re-failed to get window properties:", err)
			return
		}

		w.lastWindowID = windowID
		windowName = string(reply.Value)

		// WM_CLASS is two NUL-terminated strings: instance, class.
		if reply, err = xproto.GetProperty(w.conn, false, windowID, w.classAtom,
			xproto.GetPropertyTypeAny, 0, (1<<32)-1).Reply(); err == nil {
			wmClass = string(reply.Value)
			if idx := bytes.IndexByte(reply.Value, 0); idx >= 0 {
				wmClass = string(reply.Value[idx+1:])
			}
		}
	}

	if *debugMode {
		fmt.Println("Active window title:", windowName, "wm_class:", wmClass)
	}
	if w.prevWindowName != windowName || w.prevWMClass != wmClass {
		w.prevWindowName = windowName
		w.prevWMClass = wmClass

		w.loadWindowConfiguration(windowName, wmClass)
	}
}

func (w *watcher) loadWindowConfiguration(windowName, wmClass string) {
	if loadedConfiguration == nil {
		fmt.Println("Window switched, but no configuration:", windowName, wmClass)
		return
	}

	for _, conf := range loadedConfiguration.Apps {
		if *debugMode {
			fmt.Println("Testing title:", windowName, "wm_class:", wmClass)
		}
		if conf.matchesWindow(windowName, wmClass) {
			if *debugMode {
				fmt.Printf("Switching configuration for app %q\n", conf.Name)
			}
			currentConfiguration = conf
			return
		}
	}

	if !*debugMode {
		currentConfiguration = nil
	} else {
		fmt.Println("Keeping previous config even if window changed")
	}
}

// reapplyWindow re-resolves the active window against the (re)loaded
// configuration, using the last window title/wm_class this watcher saw. It is
// called after a config reload so currentConfiguration points at the matching
// app in the fresh config. If no window was ever seen it does nothing.
func (w *watcher) reapplyWindow() {
	if w.prevWindowName == "" && w.prevWMClass == "" {
		return
	}
	w.loadWindowConfiguration(w.prevWindowName, w.prevWMClass)
}
