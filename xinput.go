package main

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"time"
)

var xinputDevices = []*regexp.Regexp{
	regexp.MustCompile(`↳ Contour Design ShuttlePRO v2\s+id=(\d+)\s`),
}

func disableXInputPointer() {
	for {
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
			fmt.Println("Disabling XInput id:", id)
			if err := exec.Command("xinput", "disable", id).Run(); err != nil {
				log.Println("Couldn't disable xinput device:", err)
				goto end
			}
		}

	end:
		time.Sleep(60 * time.Second)
	}
}
