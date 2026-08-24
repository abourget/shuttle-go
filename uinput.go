package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"sync"
	"time"

	virtual_device "github.com/jbdemonte/virtual-device"
	"github.com/jbdemonte/virtual-device/linux"
)

// mouseButtonCodes maps a /click button name (symbolic or hex) to an EV_KEY
// button code, for the mapper's /click handler.
var mouseButtonCodes = map[string]int{
	"left":    0x110,
	"right":   0x111,
	"middle":  0x112,
	"forward": 0x117,
	"back":    0x118,
	"side":    0x119,
	"extr":    0x11a,
	"0x00":    0x110,
	"0x01":    0x111,
	"0x02":    0x112,
	"0x03":    0x119,
	"0x04":    0x11a,
	"0x05":    0x117,
	"0x06":    0x118,
}

// uinputDevice is a pair of virtual input devices backed by the
// github.com/jbdemonte/virtual-device library: a keyboard (for key events) and
// a dedicated mouse (for relative motion and button clicks). It exposes the
// small eventEmitter surface the mapper uses (KeyTap/KeyHold/KeyRelease/Type/
// MouseMove/Click) plus Destroy, and is created by newUinputDevice.
//
// The previous hand-rolled uinput code created a correctly-configured device
// (verified with evtest) yet the kernel dropped every emitted event on this
// machine. The jbdemonte/virtual-device library emits events that do reach the
// kernel, so we now delegate all emission to it. Keyboard and mouse events are
// split across two devices, mirroring the library's own keyboard/mouse
// examples, because mixing mouse buttons into a keyboard device did not deliver
// mouse events.
type uinputDevice struct {
	keyboard virtual_device.VirtualDevice
	mouse    virtual_device.VirtualDevice
	// keyMu serializes keyboard key events. The main dispatch loop and the
	// shuttle-repeat ticker goroutine can both tap keys; without a lock their
	// press/syn/release sequences could interleave and corrupt the key state.
	keyMu sync.Mutex
}

// newUinputDevice creates and registers a virtual keyboard (full EV_KEY range so
// any bound key can be emitted) and a dedicated virtual mouse (relative X/Y,
// wheel, and the standard buttons).
func newUinputDevice() (*uinputDevice, error) {
	// Full keyboard range (KEY_RESERVED+1 .. KEY_MAX) so any key can be tapped.
	keys := make([]linux.Key, 0, 0x2ff)
	for k := linux.Key(1); k <= linux.Key(0x2ff); k++ {
		keys = append(keys, k)
	}

	keyboard := virtual_device.NewVirtualDevice().
		WithBusType(linux.BUS_USB).
		WithVendor(0x0001).
		WithProduct(0x0001).
		WithVersion(0x0100).
		WithName("shuttle-go-virtual").
		WithKeys(keys)

	if err := keyboard.Register(); err != nil {
		return nil, fmt.Errorf("registering virtual keyboard: %w", err)
	}

	mouse := virtual_device.NewVirtualDevice().
		WithBusType(linux.BUS_USB).
		WithVendor(0x0001).
		WithProduct(0x0002).
		WithVersion(0x0100).
		WithName("shuttle-go-virtual-mouse").
		WithButtons([]linux.Button{
			linux.BTN_LEFT,
			linux.BTN_RIGHT,
			linux.BTN_MIDDLE,
			linux.BTN_SIDE,
			linux.BTN_EXTRA,
			linux.BTN_FORWARD,
			linux.BTN_BACK,
		}).
		WithRelAxes([]linux.RelativeAxis{linux.REL_X, linux.REL_Y, linux.REL_WHEEL})

	if err := mouse.Register(); err != nil {
		keyboard.Unregister()
		return nil, fmt.Errorf("registering virtual mouse: %w", err)
	}

	return &uinputDevice{keyboard: keyboard, mouse: mouse}, nil
}

// Destroy unregisters and closes both virtual devices.
func (d *uinputDevice) Destroy() {
	if d == nil {
		return
	}
	if d.keyboard != nil {
		d.keyboard.Unregister()
		d.keyboard = nil
	}
	if d.mouse != nil {
		d.mouse.Unregister()
		d.mouse = nil
	}
}

// KeyTap presses and releases each keycode in order, with a SYN after each
// press and after each release.
func (d *uinputDevice) KeyTap(codes []int) error {
	d.keyMu.Lock()
	defer d.keyMu.Unlock()
	for _, code := range codes {
		d.keyboard.PressKey(linux.Key(code))
		d.keyboard.SyncReport()
	}
	for i := len(codes) - 1; i >= 0; i-- {
		d.keyboard.ReleaseKey(linux.Key(codes[i]))
		d.keyboard.SyncReport()
	}
	return nil
}

// KeyHold presses each keycode (value 1) and SYNs.
func (d *uinputDevice) KeyHold(codes []int) error {
	d.keyMu.Lock()
	defer d.keyMu.Unlock()
	for _, code := range codes {
		d.keyboard.PressKey(linux.Key(code))
	}
	d.keyboard.SyncReport()
	return nil
}

// KeyRelease releases each keycode (value 0) and SYNs.
func (d *uinputDevice) KeyRelease(codes []int) error {
	d.keyMu.Lock()
	defer d.keyMu.Unlock()
	for _, code := range codes {
		d.keyboard.ReleaseKey(linux.Key(code))
	}
	d.keyboard.SyncReport()
	return nil
}

// Type emits each rune of text. Letters and unshifted symbols are tapped
// directly; symbols that require Shift (e.g. '!', '@', '~') are emitted with
// Shift held for the duration of the key.
func (d *uinputDevice) Type(text string) error {
	for _, r := range text {
		code, shift, err := runeKeyCode(r)
		if err != nil {
			return err
		}
		if !shift {
			if err := d.KeyTap([]int{code}); err != nil {
				return err
			}
			continue
		}
		// Press Shift, press the key, release the key, release Shift.
		d.keyboard.PressKey(linux.KEY_LEFTSHIFT)
		d.keyboard.PressKey(linux.Key(code))
		d.keyboard.SyncReport()
		time.Sleep(20 * time.Millisecond)
		d.keyboard.ReleaseKey(linux.Key(code))
		d.keyboard.ReleaseKey(linux.KEY_LEFTSHIFT)
		d.keyboard.SyncReport()
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// MouseMove moves the pointer by (dx, dy) relative units, or to the absolute
// screen position (x, y) when abs is true.
//
// The absolute case is implemented as a relative move: we read the current
// pointer position and emit a relative move of (x-currentX, y-currentY). This
// is used instead of the uinput ABS axes because the input layer on this
// machine does not trust the virtual device's absolute axis, so ABS events are
// ignored. If the position cannot be read, the move degrades to a relative move
// of the raw (x, y).
func (d *uinputDevice) MouseMove(dx, dy int, abs bool) error {
	if abs {
		if x, y, err := peekMousePos(); err == nil {
			dx -= x
			dy -= y
		}
	}
	d.mouse.SendRelativeEvent(linux.REL_X, int32(dx))
	d.mouse.SendRelativeEvent(linux.REL_Y, int32(dy))
	d.mouse.SyncReport()
	return nil
}

// peekMousePos returns the current pointer position. On Wayland it first asks
// the Shuttle Pro GNOME extension over D-Bus, which reports the real cursor
// position; if that is unavailable it falls back to peeking the mouse interface
// (/dev/input/mice), which only reports relative motion.
func peekMousePos() (x, y int, err error) {
	if isWayland() {
		if info, ok := getWaylandInfo(); ok {
			return info.X, info.Y, nil
		}
	}
	return peekMicePos()
}

// peekMicePos returns the current pointer position by peeking the latest event
// queued on the mouse interface (/dev/input/mice). That interface reports only
// relative motion, so the newest queued event is the pointer's current
// position; if the queue is empty the pointer is idle at its last known
// position, which we report as (0, 0). The peek is non-blocking (O_NONBLOCK),
// so it never waits for a new event and never consumes one.
func peekMicePos() (x, y int, err error) {
	f, err := os.Open("/dev/input/mice")
	if err != nil {
		return 0, 0, fmt.Errorf("opening /dev/input/mice: %s", err)
	}
	defer f.Close()

	if err := syscall.SetNonblock(int(f.Fd()), true); err != nil {
		return 0, 0, fmt.Errorf("setting /dev/input/mice nonblocking: %s", err)
	}

	// A single 7-byte mouseevent: 3 status bytes + x, y, z as int16
	// little-endian.
	buf := make([]byte, 7)
	n, rerr := f.Read(buf)
	if n < 7 {
		// No event queued (EAGAIN) or a short read: the pointer is idle at its
		// last known position.
		if rerr != nil && rerr != syscall.EAGAIN {
			return 0, 0, fmt.Errorf("reading /dev/input/mice: %s", rerr)
		}
		return 0, 0, nil
	}
	x = int(int16(binary.LittleEndian.Uint16(buf[3:5])))
	y = int(int16(binary.LittleEndian.Uint16(buf[5:7])))
	return x, y, nil
}

// Click presses and releases the given EV_KEY button code repeats times.
func (d *uinputDevice) Click(code int, repeats int) error {
	if repeats < 1 {
		repeats = 1
	}
	for i := 0; i < repeats; i++ {
		d.mouse.PressButton(linux.Button(code))
		d.mouse.SyncReport()
		time.Sleep(20 * time.Millisecond)
		d.mouse.ReleaseButton(linux.Button(code))
		d.mouse.SyncReport()
	}
	return nil
}
