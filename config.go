package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hypebeast/go-osc/osc"
)

var loadedConfiguration = &Config{}
var currentConfiguration *AppConfig

type Config struct {
	Apps []*AppConfig `json:"apps"`
}

type AppConfig struct {
	Name               string   `json:"name"`
	MatchWindowTitles  []string `json:"match_window_titles"`
	SlowJog            *int     `json:"slow_jog"` // Time in millisecond to use slow jog
	Driver             string   `json:"driver"`
	windowTitleRegexps []*regexp.Regexp
	Bindings           map[string]string `json:"bindings"`
	bindings           []*deviceBinding
}

func (ac *AppConfig) parse() error {
	if len(ac.MatchWindowTitles) == 0 {
		ac.windowTitleRegexps = []*regexp.Regexp{
			regexp.MustCompile(`.*`),
		}
		return nil
	}

	for _, window := range ac.MatchWindowTitles {
		re, err := regexp.Compile(window)
		if err != nil {
			return fmt.Errorf("Invalid regexp in window match %q: %s", window, err)
		}

		ac.windowTitleRegexps = append(ac.windowTitleRegexps, re)
	}

	return nil
}

type deviceBinding struct {
	rawKey   string
	rawValue string

	// Input
	heldButtons map[int]bool
	buttonDown  int
	otherKey    string

	driver    string
	oscClient *osc.Client

	// Output
	holdButtons []string
	pressButton string
	original    string
	description string
}

func (ac *AppConfig) parseBindings() error {
	driverProtocol := "xdotool"
	var oscClient *osc.Client

	switch {
	case ac.Driver == "":
	case ac.Driver == "exec":
		driverProtocol = "exec"
	case ac.Driver == "xdotool":
	case strings.HasPrefix(ac.Driver, "osc://"):
		addr, err := url.Parse(ac.Driver)
		if err != nil {
			return fmt.Errorf("failed parsing osc:// address: %s", err)
		}
		hostParts := strings.Split(addr.Host, ":")
		if len(hostParts) != 2 {
			return fmt.Errorf("please specify a port for the osc:// address")
		}
		port, _ := strconv.ParseInt(hostParts[1], 10, 32)

		driverProtocol = "osc"
		oscClient = osc.NewClient(hostParts[0], int(port))
	default:
		return fmt.Errorf(`invalid driver %q, use one of: "xdotool" (default), "exec", "osc://address:port"`, ac.Driver)
	}

	for key, value := range ac.Bindings {
		if strings.HasPrefix(key, "_") {
			continue
		}

		binding, description := bindingAndDescription(driverProtocol, value)
		newBinding := &deviceBinding{heldButtons: make(map[int]bool), rawKey: key, rawValue: value, original: binding, description: description, driver: driverProtocol, oscClient: oscClient}

		// Input
		input := strings.Split(key, "+")
		for idx, part := range input {
			cleanPart := strings.TrimSpace(part)
			key := strings.ToUpper(cleanPart)
			if shuttleKeys[key] == 0 && !otherShuttleKeysUpper[key] {
				return fmt.Errorf("invalid shuttle device key map: %q doesn't exist", cleanPart)
			}
			if idx == len(input)-1 {
				if shuttleKeys[key] != 0 {
					newBinding.buttonDown = shuttleKeys[key]
				} else {
					newBinding.otherKey = key
				}
			} else {
				keyID := shuttleKeys[key]
				if keyID == 0 {
					return fmt.Errorf("binding %q, expects a button press, not a shuttle or jog movement", key)
				}
				newBinding.heldButtons[keyID] = true
			}
		}

		// Output
		// output := strings.Split(value, "+")
		// for idx, part := range output {
		// 	cleanPart := strings.TrimSpace(part)
		// 	buttonName := strings.ToUpper(cleanPart)
		// 	if keyboardKeysUpper[buttonName] == 0 {
		// 		return fmt.Errorf("keyboard key unknown: %q", cleanPart)
		// 	}
		// 	if idx == len(output)-1 {
		// 		newBinding.pressButton = buttonName
		// 	} else {
		// 		newBinding.holdButtons = append(newBinding.holdButtons, buttonName)
		// 	}
		// }

		ac.bindings = append(ac.bindings, newBinding)

		if *debugMode {
			fmt.Printf("BINDING: %#v\n", newBinding)
		}
	}

	return nil
}

var xdoDescriptionRE = regexp.MustCompile(`([^/]*)(\s*// *(.+))?`)
var oscDescriptionRE = regexp.MustCompile(`([^#]*)(\s*# *(.+))?`)

func bindingAndDescription(protocol, input string) (string, string) {
	re := xdoDescriptionRE
	if protocol == "osc" || protocol == "exec" {
		re = oscDescriptionRE
	}

	matches := re.FindStringSubmatch(input)
	if matches == nil {
		return input, ""
	}
	return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[3])
}

func LoadConfig(filename string) error {
	cnt, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	newConfig := &Config{}
	err = json.Unmarshal(cnt, &newConfig)
	if err != nil {
		return err
	}

	for _, app := range newConfig.Apps {
		if err := app.parse(); err != nil {
			return fmt.Errorf("Error parsing app %q's matchers: %s", app.Name, err)
		}

		if err := app.parseBindings(); err != nil {
			return fmt.Errorf("Error parsing app %q's bindings: %s", app.Name, err)
		}

	}

	loadedConfiguration = newConfig

	return nil
}
