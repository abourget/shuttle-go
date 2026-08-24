package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hypebeast/go-osc/osc"
)

// bindingValue is a binding's value. It may be written three ways in the
// config:
//
//	"a plain string"      -> a single key/command (Key)
//	["cmd", "cmd", ...]   -> a macro sequence (Macros)
//	{ "key": ..., ... }    -> an object with per-binding knobs (see fields)
//
// The object form is the only one that can express repeat/delay/once; the
// string and array forms decode to it with sensible defaults.
type bindingValue struct {
	Key          string   `json:"key"`
	Repeat       int      `json:"repeat"`
	DelayMS      int      `json:"delay_ms"`
	StartDelayMS int      `json:"start_delay_ms"`
	Once         bool     `json:"once"`
	Macros       []string `json:"macros"`
	// Comment is an optional human note for the binding (e.g. "Play"). In the
	// string/array forms the note is embedded in the value after "//" or "#";
	// in the object form it lives here so porting a binding to the object form
	// does not lose the note.
	Comment string `json:"comment"`
}

var loadedConfiguration = &Config{}
var currentConfiguration *AppConfig

type Config struct {
	Apps []*AppConfig `json:"apps"`
}

type AppConfig struct {
	Name               string   `json:"name"`
	MatchWindowTitles  []string `json:"match_window_titles"`
	MatchWMClass       []string `json:"match_wm_class"`
	SlowJog            *int     `json:"slow_jog"` // Time in millisecond to use slow jog
	Driver             string   `json:"driver"`
	windowTitleRegexps []*regexp.Regexp
	wmClassRegexps     []*regexp.Regexp
	Bindings           map[string]json.RawMessage `json:"bindings"`
	bindings           []*deviceBinding
}

func (ac *AppConfig) parse() error {
	if len(ac.MatchWindowTitles) == 0 && len(ac.MatchWMClass) == 0 {
		ac.windowTitleRegexps = []*regexp.Regexp{
			regexp.MustCompile(`.*`),
		}
		return nil
	}

	for _, window := range ac.MatchWindowTitles {
		re, err := regexp.Compile(window)
		if err != nil {
			return fmt.Errorf("Invalid regexp in window title match %q: %s", window, err)
		}
		ac.windowTitleRegexps = append(ac.windowTitleRegexps, re)
	}

	for _, class := range ac.MatchWMClass {
		re, err := regexp.Compile(class)
		if err != nil {
			return fmt.Errorf("Invalid regexp in wm_class match %q: %s", class, err)
		}
		ac.wmClassRegexps = append(ac.wmClassRegexps, re)
	}

	return nil
}

// matchesWindow reports whether the given window matches this app's configured
// matchers. Within each dimension, any regexp matching is enough (OR). When
// both match_window_titles and match_wm_class are present, both dimensions must
// match (AND), so e.g. you can target one specific Emacs window but not all of
// them.
func (ac *AppConfig) matchesWindow(title, wmClass string) bool {
	if len(ac.windowTitleRegexps) > 0 && len(ac.wmClassRegexps) > 0 {
		return ac.matchAny(ac.windowTitleRegexps, title) && ac.matchAny(ac.wmClassRegexps, wmClass)
	}
	if len(ac.windowTitleRegexps) > 0 {
		return ac.matchAny(ac.windowTitleRegexps, title)
	}
	if len(ac.wmClassRegexps) > 0 {
		return ac.matchAny(ac.wmClassRegexps, wmClass)
	}
	// No matchers configured: match everything.
	return true
}

func (ac *AppConfig) matchAny(regexps []*regexp.Regexp, value string) bool {
	for _, re := range regexps {
		if re.MatchString(value) {
			return true
		}
	}
	return false
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
	holdButtons  []string
	pressButton  string
	original     string
	description  string
	macros       []string
	once         bool
	repeat       int
	delayMS      int
	startDelayMS int
}

func (ac *AppConfig) parseBindings() error {
	driverProtocol := "uinput"
	var oscClient *osc.Client

	switch {
	case ac.Driver == "":
	case ac.Driver == "exec":
		driverProtocol = "exec"
	case ac.Driver == "uinput":
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
		return fmt.Errorf(`invalid driver %q, use one of: "uinput" (default), "exec", "osc://address:port"`, ac.Driver)
	}

	for key, raw := range ac.Bindings {
		if strings.HasPrefix(key, "_") {
			continue
		}

		bv, err := decodeBindingValue(raw)
		if err != nil {
			return fmt.Errorf("binding %q: %s", key, err)
		}

		binding, description := bindingAndDescription(driverProtocol, bv.Key)
		// An explicit "comment" in the object form wins over a note embedded in
		// the key string, so porting a binding to the object form keeps its note.
		if c := strings.TrimSpace(bv.Comment); c != "" {
			description = c
		}
		newBinding := &deviceBinding{heldButtons: make(map[int]bool), rawKey: key, rawValue: bv.Key, original: binding, description: description, driver: driverProtocol, oscClient: oscClient, macros: bv.Macros, once: bv.Once, repeat: bv.Repeat, delayMS: bv.DelayMS, startDelayMS: bv.StartDelayMS}

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

// decodeBindingValue interprets a binding's raw JSON value. It may be a plain
// string (a single key/command), an array of strings (a macro sequence), or an
// object with the per-binding knobs (repeat, delay_ms, start_delay_ms, once,
// key, macros). Defaults are applied for any field left unset.
func decodeBindingValue(raw json.RawMessage) (bindingValue, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return bindingValue{}, fmt.Errorf("empty binding value")
	}

	var bv bindingValue
	switch {
	case strings.HasPrefix(trimmed, "["):
		// Array of strings -> macro sequence.
		var items []string
		if err := json.Unmarshal(raw, &items); err != nil {
			return bindingValue{}, fmt.Errorf("expected a string, an object, or an array of strings: %s", err)
		}
		bv = bindingValue{Macros: items}
	case strings.HasPrefix(trimmed, "{"):
		// Object form with the per-binding knobs.
		if err := json.Unmarshal(raw, &bv); err != nil {
			return bindingValue{}, fmt.Errorf("invalid binding object: %s", err)
		}
	default:
		// Plain string -> a single key/command.
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return bindingValue{}, fmt.Errorf("expected a string, an object, or an array of strings: %s", err)
		}
		bv = bindingValue{Key: s}
	}

	// Apply defaults for any knob left unset.
	if bv.Repeat <= 0 {
		bv.Repeat = 1
	}
	if bv.DelayMS <= 0 {
		bv.DelayMS = 25
	}
	if bv.StartDelayMS < 0 {
		bv.StartDelayMS = 0
	}
	return bv, nil
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

	if err := checkDuplicateBindings(cnt); err != nil {
		return err
	}

	loadedConfiguration = newConfig

	return nil
}

// usesOSCDriver reports whether any loaded binding uses the "osc" driver, i.e.
// whether the config references an osc:// address. The OSC feedback listener
// only needs to run when at least one binding sends OSC, so this gates its
// startup.
func usesOSCDriver() bool {
	if loadedConfiguration == nil {
		return false
	}
	for _, app := range loadedConfiguration.Apps {
		for _, b := range app.bindings {
			if b.driver == "osc" {
				return true
			}
		}
	}
	return false
}

// checkDuplicateBindings reports any app whose "bindings" object repeats a key.
// Go's map[string]json.RawMessage keeps only one value per key, so a duplicate
// would otherwise silently drop the earlier binding (e.g. two "JogL" entries).
// The raw JSON is decoded with a duplicate-preserving decoder so both copies are
// visible before the map collapses them.
func checkDuplicateBindings(cnt []byte) error {
	var raw struct {
		Apps []struct {
			Name     string          `json:"name"`
			Bindings json.RawMessage `json:"bindings"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(cnt, &raw); err != nil {
		return nil // LoadConfig already validated the JSON
	}

	for _, app := range raw.Apps {
		if len(app.Bindings) == 0 {
			continue
		}
		if dups := duplicateBindingKeys(app.Bindings); len(dups) > 0 {
			return fmt.Errorf("app %q has duplicate binding keys: %s (a key can only appear once; the earlier value is silently dropped)", app.Name, strings.Join(dups, ", "))
		}
	}
	return nil
}

// duplicateBindingKeys decodes a bindings object as a stream of key/value
// pairs (preserving duplicate keys, which a map would collapse) and returns
// the keys that appear more than once, in first-seen order.
func duplicateBindingKeys(raw json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, _ := dec.Token()
	if tok != json.Delim('{') {
		return nil
	}

	var keys []string
	for dec.More() {
		// The object key is a JSON string literal, so Token returns its value
		// directly; do not also Decode it (that would consume the value).
		tok, _ := dec.Token()
		k, ok := tok.(string)
		if !ok {
			return nil
		}
		keys = append(keys, k)
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			return nil
		}
	}

	seen := make(map[string]int)
	var dups []string
	for _, k := range keys {
		seen[k]++
		if seen[k] == 2 {
			dups = append(dups, k)
		}
	}
	return dups
}
