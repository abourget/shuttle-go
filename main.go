package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gvalkov/golang-evdev"
)

var configFile = flag.String("config", filepath.Join(os.Getenv("HOME"), ".shuttle-go.json"), "Location to the .shuttle-go.json configuration")
var debugMode = flag.Bool("debug", false, "Show debug messages (like window titles)")
var logFile = flag.String("log-file", "", "Log to a file instead of stdout")

func main() {
	flag.Parse()

	if *logFile != "" {
		log, err := os.Create(*logFile)
		if err != nil {
			os.Exit(101)
		}
		defer log.Close()
		os.Stderr = log
		os.Stdout = log
	}

	devicePath := "/dev/input/by-id/usb-Contour_Design_ShuttlePRO_v2-event-if00"
	if len(flag.Args()) == 1 {
		devicePath = flag.Arg(0)
	}
	fmt.Println("Using device", devicePath)

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

	// Shuttle device event receiver
	dev, err := evdev.Open(devicePath)
	if err != nil {
		fmt.Println("Couldn't open Shuttle device:", err)
		os.Exit(2)
	}

	fmt.Println("Ready")
	mapper := NewMapper(dev)
	mapper.watcher = watcher

	// IF there's an `osc` driver specified, launch an OSC listener too:
	go listenOSCFeedback()

	for {
		if err := mapper.Process(); err != nil {
			fmt.Println("Error processing input events (continuing):", err)
			os.Exit(123)
		}
	}

}
