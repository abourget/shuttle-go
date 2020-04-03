package main

import (
	"fmt"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgb/xtest"
)

type watcher struct {
	conn                 *xgb.Conn
	root                 xproto.Window
	activeAtom, nameAtom xproto.Atom
	prevWindowName       string
	lastWindowID         xproto.Window
}

func NewWindowWatcher() *watcher {
	return &watcher{}
}

func (w *watcher) Setup() error {
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

	w.activeAtom = activeAtom.Atom
	w.nameAtom = nameAtom.Atom

	return nil
}

func (w *watcher) Run() {
	for {
		w.watch()
		time.Sleep(2 * time.Second)
	}
}

func (w *watcher) watch() {
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

	windowName := string(reply.Value)
	if w.prevWindowName != windowName {
		w.prevWindowName = windowName

		w.loadWindowConfiguration(windowName)
	}
}

func (w *watcher) loadWindowConfiguration(windowName string) {
	if loadedConfiguration == nil {
		fmt.Println("Window name switched, but no configuration:", windowName)
		return
	}

	for _, conf := range loadedConfiguration.Apps {
		for _, re := range conf.windowTitleRegexps {
			if *debugMode {
				fmt.Println("Testing title:", windowName)
			}
			if re.MatchString(windowName) {
				fmt.Printf("Switching configuration for app %q\n", conf.Name)
				currentConfiguration = conf
				return
			}
		}
	}

	if !*debugMode {
		currentConfiguration = nil
	} else {
		fmt.Println("Keeping previous config even if window changed")
	}
}
