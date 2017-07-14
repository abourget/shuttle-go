package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bendahl/uinput"
	"github.com/gvalkov/golang-evdev"
)

var configFile = flag.String("config", filepath.Join(os.Getenv("HOME"), ".shuttle-go.json"), "Location to the .shuttle-go.json configuration")

func main() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Println("Missing device name as parameter.\nExample: [program] /dev/input/by-id/usb-Contour_Design_ShuttlePRO_v2-event-if00\n")
		os.Exit(1)
	}

	err := LoadConfig(*configFile)
	if err != nil {
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

	// Virtual keyboard
	vk, err := uinput.CreateKeyboard("/dev/uinput", []byte("Go Virtual Shuttle Pro V2"))
	if err != nil {
		log.Println("Can't open dev:", err)
	}

	// Shuttle device event receiver
	dev, err := evdev.Open(flag.Arg(0))
	if err != nil {
		fmt.Println("Couldn't open Shuttle device:", err)
		os.Exit(2)
	}

	fmt.Println("ready")
	mapper := NewMapper(vk, dev)
	mapper.watcher = watcher
	for {
		if err := mapper.Process(); err != nil {
			fmt.Println("Error processing input events (continuing):", err)
		}
	}

}
