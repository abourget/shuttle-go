package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"
)

var xinputDevices = []*regexp.Regexp{
	regexp.MustCompile(`↳ Contour Design ShuttlePRO v2\s+id=(\d+)\s`),
}

var libinputDevices = []*regexp.Regexp{
	regexp.MustCompile(`path: /dev/input/(\S+)\s+.*ShuttlePRO`),
}

func isWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

func disableXInputPointer() {
	for {
		if isWayland() {
			// On Wayland, use libinput to disable the shuttle pointer
			out, err := exec.Command("libinput", "list-devices").Output()
			if err != nil {
				log.Println("Couldn't list libinput devices:", err)
				goto end
			}

			for _, dev := range libinputDevices {
				matches := dev.FindStringSubmatch(string(out))
				if matches == nil {
					continue
				}

			devicePath := "/dev/input/" + matches[1]
			if *debugMode {
				fmt.Println("Disabling libinput device:", devicePath)
			}
				if err := exec.Command("libinput", "disable", devicePath).Run(); err != nil {
					log.Println("Couldn't disable libinput device:", err)
					goto end
				}
			}
		} else {
			// On X11, use xinput to disable the shuttle pointer
			cnt, err := exec.Command("xinput", "list").Output()
			if err != nil {
				log.Println("Couldn't list xinput:", err)
				goto end
			}

			for _, dev := range xinputDevices {
				matches := dev.FindStringSubmatch(string(cnt))
				if matches == nil {
					continue
				}

			id := matches[1]
			if *debugMode {
				fmt.Println("Disabling XInput id:", id)
			}
				if err := exec.Command("xinput", "disable", id).Run(); err != nil {
					log.Println("Couldn't disable xinput device:", err)
					goto end
				}
			}
		}

	end:
		time.Sleep(60 * time.Second)
	}
}
