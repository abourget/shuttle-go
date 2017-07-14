package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

var loadedConfiguration = &Config{}
var currentConfiguration *AppConfig

type Config struct {
	Apps []*AppConfig `json:"apps"`
}

type AppConfig struct {
	Name               string   `json:"name"`
	MatchWindowTitles  []string `json:"match_window_titles"`
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
	// Input
	heldButtons map[int]bool
	buttonDown  int
	otherKey    string

	// Output
	holdButtons []string
	pressButton string
}

func (ac *AppConfig) parseBindings() error {
	for key, value := range ac.Bindings {
		newBinding := &deviceBinding{heldButtons: make(map[int]bool)}

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
					return fmt.Errorf("binding %q, expects a button press, not a shuttle or jog movement")
				}
				newBinding.heldButtons[keyID] = true
			}
		}

		// Output
		output := strings.Split(value, "+")
		for idx, part := range output {
			cleanPart := strings.TrimSpace(part)
			buttonName := strings.ToUpper(cleanPart)
			if keyboardKeysUpper[buttonName] == 0 {
				return fmt.Errorf("keyboard key unknown: %q", cleanPart)
			}
			if idx == len(output)-1 {
				newBinding.pressButton = buttonName
			} else {
				newBinding.holdButtons = append(newBinding.holdButtons, buttonName)
			}
		}

		ac.bindings = append(ac.bindings, newBinding)

		fmt.Printf("BINDING: %#v\n", newBinding)
	}

	return nil
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
