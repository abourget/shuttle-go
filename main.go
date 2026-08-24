package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gvalkov/golang-evdev"
)

var configFile = flag.String("config", filepath.Join(os.Getenv("HOME"), ".shuttle-go.json"), "Location to the .shuttle-go.json configuration")
var debugMode = flag.Bool("debug", false, "Show debug messages (like window titles)")
var verboseMode = flag.Bool("verbose", false, "For each held key, print which key is held and the output it is sending (to debug key-repeat timing)")
var logFile = flag.String("log-file", "", "Log to a file instead of stdout")
var showVersion = flag.Bool("version", false, "Print the semantic version and exit")
var testEmit = flag.String("test", "", "Emit a known key sequence and exit for verifying uinput delivery: 'test' taps a,b,c,d,e; 'hold' presses one key, holds it 3s, then releases it; 'tap' taps the same key 5x at 200ms (like the shuttle ticker)")
var holdDevice = flag.Bool("hold", false, "Create the uinput device, report the grab state, and keep it alive until interrupted")

// version is the semantic version, set at build time via
// -ldflags "-X main.version=..." from the committed VERSION file.
var version = "unknown"

// waitForDevice blocks until the Shuttle device can be opened, retrying every
// 5 seconds. It returns a freshly opened device (so a re-plugged device gets a
// clean file handle).
func waitForDevice(devicePath string) *evdev.InputDevice {
	for {
		dev, err := evdev.Open(devicePath)
		if err == nil {
			return dev
		}
		fmt.Println("Shuttle device not available:", err)
		fmt.Println("Waiting for the device to be plugged in (retrying every 5s)...")
		time.Sleep(5 * time.Second)
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// --test: create the uinput device and emit a known key sequence, then
	// exit. This lets you verify event delivery with an independent reader
	// (e.g. `evtest`) without touching the Shuttle. The default sequence is
	// a, b, c, d, e (key codes 30, 48, 46, 32, 18).
	if *testEmit != "" {
		uinput, err := newUinputDevice()
		if err != nil {
			fmt.Println("Error creating uinput device:", err)
			os.Exit(11)
		}
		defer uinput.Destroy()
		fmt.Println("uinput device ready (test mode)")
		fmt.Println("Now run `evtest` in another terminal, select the shuttle-go-virtual device, and press Enter to start reading.")
		fmt.Println("Waiting 10s before emitting so you have time to select the device and start evtest...")
		time.Sleep(10 * time.Second)

		if *testEmit == "hold" {
			// Diagnostic for "key appears to auto-repeat": emit ONE key, hold it
			// down for 3s, then release it. In evtest you should see exactly two
			// EV_KEY events for code 30: value 1 (down) and, ~3s later, value 0
			// (up). If the value-0 event never appears, the release is being lost
			// and the key is stuck down (which the OS then auto-repeats).
			fmt.Println("Emitting a single 'a' (code 30): press, hold 3s, release.")
			if err := uinput.KeyHold([]int{30}); err != nil {
				fmt.Println("Error pressing test key:", err)
				os.Exit(12)
			}
			fmt.Println("  pressed (EV_KEY 30 value 1). Holding for 3s...")
			time.Sleep(3 * time.Second)
			if err := uinput.KeyRelease([]int{30}); err != nil {
				fmt.Println("Error releasing test key:", err)
				os.Exit(12)
			}
			fmt.Println("  released (EV_KEY 30 value 0).")
			fmt.Println("Holding the device for 10s more so evtest can keep reading...")
			time.Sleep(10 * time.Second)
			fmt.Println("Done. Did evtest show BOTH the value-1 and the later value-0 for code 30?")
			return
		}

		if *testEmit == "tap" {
			// Mimic the shuttle-repeat ticker: tap the SAME key (a, code 30)
			// several times at a fixed 200ms interval. Each tap is a clean
			// press+release. In evtest you should see five short down/up pairs
			// for code 30 and NO run of repeated 'a's. If you instead see a long
			// auto-repeat run, a tap is being held down long enough to trigger it.
			fmt.Println("Tapping 'a' (code 30) 5 times, 200ms apart (like the shuttle ticker).")
			for i := 0; i < 5; i++ {
				if err := uinput.KeyTap([]int{30}); err != nil {
					fmt.Println("Error tapping test key:", err)
					os.Exit(12)
				}
				fmt.Printf("  tap %d (EV_KEY 30 down+up)\n", i+1)
				if i < 4 {
					time.Sleep(200 * time.Millisecond)
				}
			}
			fmt.Println("Holding the device for 10s more so evtest can keep reading...")
			time.Sleep(10 * time.Second)
			fmt.Println("Done. Did you see five clean down/up pairs, or a long auto-repeat run of 'a's?")
			return
		}

		codes := []int{30, 48, 46, 32, 18} // a, b, c, d, e
		for i, c := range codes {
			if err := uinput.KeyTap([]int{c}); err != nil {
				fmt.Println("Error emitting test key:", err)
				os.Exit(12)
			}
			fmt.Printf("emitted test key %d (%c)\n", c, "abcde"[i])
			time.Sleep(1 * time.Second)
		}
		fmt.Println("Keys emitted. Holding the device for 10s more so evtest can keep reading...")
		time.Sleep(10 * time.Second)
		fmt.Println("Done. If evtest showed EV_KEY events for codes 30,48,46,32,18, delivery is working.")
		return
	}

	// --hold: create the uinput device, report the grab state, then keep it
	// alive (emitting nothing) until interrupted. Use this to run a long-lived
	// device while you inspect it with evtest or check who is grabbing it.
	if *holdDevice {
		uinput, err := newUinputDevice()
		if err != nil {
			fmt.Println("Error creating uinput device:", err)
			os.Exit(11)
		}
		defer uinput.Destroy()
		fmt.Println("uinput device ready (hold mode); it will stay alive until you press Ctrl-C")
		fmt.Println("Inspect it now (e.g. `sudo evtest`, select the shuttle-go-virtual node).")
		select {}
	}

	if *logFile != "" {
		log, err := os.Create(*logFile)
		if err != nil {
			os.Exit(101)
		}
		defer log.Close()
		os.Stderr = log
		os.Stdout = log
	}

	devicePath := "/dev/input/by-id/usb-Contour_Design_ShuttlePRO_v2-event-mouse"
	if len(flag.Args()) == 1 {
		devicePath = flag.Arg(0)
	}
	if *debugMode {
		fmt.Println("Using device", devicePath)
	}

	if err := LoadConfig(*configFile); err != nil {
		fmt.Println("Error reading configuration:", err)
		os.Exit(10)
	}

	go disableXInputPointer()

	// X-window title change watcher
	watcher := NewWindowWatcher()
	if err := watcher.Setup(); err != nil {
		fmt.Println("Error watching X window:", err)
		os.Exit(3)
	}

	go watcher.Run()

	// Watch the config file and reload it (and re-resolve the active window)
	// whenever it changes, so edits take effect without a restart.
	go NewConfigWatcher(*configFile, watcher).Run()

	// Only launch the OSC feedback listener if some binding actually uses the
	// "osc" driver. It binds a fixed port and must run at most once for the
	// process lifetime.
	if usesOSCDriver() {
		go listenOSCFeedback()
	}

	// Create the virtual uinput device used to emit keyboard and mouse
	// events. It must exist before any binding can fire.
	uinput, err := newUinputDevice()
	if err != nil {
		fmt.Println("Error creating uinput device:", err)
		os.Exit(11)
	}
	defer uinput.Destroy()
	if *debugMode {
		fmt.Println("uinput device ready")
	}

	// Wait for the device, then process its events. If the device is unplugged
	// (Process errors), release any held keys, destroy the uinput device, and
	// go back to waiting for the device to be plugged in again.
	for {
		dev := waitForDevice(devicePath)
		if *debugMode {
			fmt.Println("Ready")
		}

		mapper := NewMapper(dev, uinput)
		mapper.watcher = watcher

		for {
			if err := mapper.Process(); err != nil {
				fmt.Println("Lost the Shuttle device (unplugged?):", err)
				mapper.ReleaseAll()
				uinput.Destroy()
				uinput, err = newUinputDevice()
				if err != nil {
					fmt.Println("Error recreating uinput device:", err)
					os.Exit(11)
				}
				break
			}
		}
	}

}
