package main

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	evdev "github.com/gvalkov/golang-evdev"
	"github.com/hypebeast/go-osc/osc"
)

// eventEmitter is the subset of *uinputDevice the mapper uses to emit
// keyboard, relative-mouse and button events. It is an interface so the
// mapper can be unit-tested with a fake that records events instead of
// writing to /dev/uinput.
type eventEmitter interface {
	KeyTap(codes []int) error
	KeyHold(codes []int) error
	KeyRelease(codes []int) error
	Type(text string) error
	MouseMove(dx, dy int, abs bool) error
	Click(code int, repeats int) error
}

// Mapper receives events from the Shuttle devices, and maps (through
// configuration) to the Virtual Keyboard events.
type Mapper struct {
	inputDevice *evdev.InputDevice
	uinput      eventEmitter
	state       buttonsState
	watcher     *watcher
}

type buttonsState struct {
	jog               int
	shuttle           int
	buttonsHeld       map[int]bool
	activeBinding     map[int][]int
	activeMacroCancel map[int]context.CancelFunc
	shuttleRepeat     context.CancelFunc
	lastJog           time.Time
}

func NewMapper(inputDevice *evdev.InputDevice, uinput eventEmitter) *Mapper {
	m := &Mapper{
		inputDevice: inputDevice,
		uinput:      uinput,
	}
	m.state.buttonsHeld = make(map[int]bool)
	m.state.activeBinding = make(map[int][]int)
	m.state.activeMacroCancel = make(map[int]context.CancelFunc)
	m.state.jog = -1
	return m
}

func (m *Mapper) ReleaseAll() {
	// Stop any shuttle-position repeat that is still running.
	m.stopShuttleRepeat()
	for _, codes := range m.state.activeBinding {
		if *debugMode {
			fmt.Printf("uinput release %v\n", codes)
		}
		if *verboseMode {
			fmt.Printf("RELEASE (all) codes=%v\n", codes)
		}
		m.uinput.KeyRelease(codes)
	}
	for _, cancel := range m.state.activeMacroCancel {
		cancel()
	}
	m.state.activeMacroCancel = make(map[int]context.CancelFunc)
}

func (m *Mapper) Process() error {
	evs, err := m.inputDevice.Read()
	if err != nil {
		return err
	}

	if *debugMode {
		for _, ev := range evs {
			fmt.Printf("INPUT: TYPE: %d\tCODE: %d\tVALUE: %d\n", ev.Type, ev.Code, ev.Value)
		}
	}

	m.dispatch(evs)

	return nil
}

func (m *Mapper) dispatch(evs []evdev.InputEvent) {
	if *verboseMode {
		// Raw input trace: show every event in the batch plus the decoded
		// shuttle/jog values, so we can see how a held wheel position is
		// actually reported (e.g. whether SYN-only batches make shuttleVal
		// flap back to 0).
		parts := make([]string, 0, len(evs))
		for _, ev := range evs {
			parts = append(parts, fmt.Sprintf("t%d c%d v%d", ev.Type, ev.Code, ev.Value))
		}
		fmt.Printf("IN shuttle=%d jog=%d batch=[%s]\n", shuttleVal(evs), jogVal(evs), strings.Join(parts, ", "))
	}

	newJogVal := jogVal(evs)
	if m.state.jog != newJogVal {
		if m.state.jog != -1 {
			if m.state.lastJog.IsZero() {
				m.state.lastJog = time.Now()
			}

			// A jog is "slow" only if slow-jog is enabled (slow_jog > 0) and
			// the previous jog was more than slowJogTiming() ago. When slow_jog
			// is 0 (disabled), every jog is a normal jog.
			slow := ""
			if slowJogTiming() > 0 && time.Since(m.state.lastJog) > slowJogTiming() {
				slow = "Slow"
			}
			// Trigger JL or JR if we're advancing or not..
			delta := newJogVal - m.state.jog
			// The lookup key must match the config binding key (e.g. "JogR",
			// "SlowJogL"), which is what otherKey is set to at parse time.
			jogKey := "JogR"
			if slow != "" {
				jogKey = "SlowJogR"
			}
			if (delta > 0 || delta < -200) && (delta < 200) {
				if err := m.EmitOther(jogKey); err != nil {
					fmt.Println("Jog right:", err)
				}
			} else {
				jogKey = "JogL"
				if slow != "" {
					jogKey = "SlowJogL"
				}
				if err := m.EmitOther(jogKey); err != nil {
					fmt.Println("Jog left:", err)
				}
			}

			m.state.lastJog = time.Now()
		}
		m.state.jog = newJogVal
	}

	newShuttleVal := shuttleVal(evs)
	if m.state.shuttle != newShuttleVal {
		// Leaving the previous position: stop its repeat (if any). A held
		// position is a tap-based repeat, so there is no key to release.
		m.stopShuttleRepeat()

		keyName := fmt.Sprintf("S%d", newShuttleVal)
		if *debugMode {
			fmt.Println("SHUTTLE", keyName)
		}

		if newShuttleVal == 0 {
			// S0 is a single tap (e.g. "Stop").
			if err := m.EmitOther(keyName); err != nil {
				fmt.Printf("Shuttle movement %q: %s\n", keyName, err)
			}
		} else {
			// S-7..S7 repeat their command while the wheel stays held here.
			if err := m.EmitOtherRepeat(keyName); err != nil {
				fmt.Printf("Shuttle movement %q: %s\n", keyName, err)
			}
		}
		m.state.shuttle = newShuttleVal
	}

	for i := range evs {
		// Some Shuttle Pro variants report the M1/M2 buttons as 272/273
		// (M3/M4). Translate them onto the M1/M2 codes so a config written
		// with M1/M2 fires for either variant.
		if evs[i].Type == 1 {
			evs[i].Code = uint16(mToM1M2(int(evs[i].Code)))
		}
	}

	for _, ev := range evs {
		if ev.Type != 1 {
			continue
		}

		heldButtons, lastDown := buttonVals(m.state.buttonsHeld, ev)

		if lastDown != 0 {
			modifiers := buttonsToModifiers(heldButtons, lastDown)
			codes, err := m.EmitKeys(modifiers, lastDown)
			if err != nil {
				fmt.Println("Button press:", err)
			}
			if len(codes) > 0 {
				m.state.activeBinding[lastDown] = codes
			}
		} else if ev.Value == 0 {
			if codes, ok := m.state.activeBinding[int(ev.Code)]; ok {
				if *debugMode {
					fmt.Printf("uinput release %v\n", codes)
				}
				if *verboseMode {
					fmt.Printf("RELEASE key=%q codes=%v\n", reverseShuttleKeys[int(ev.Code)], codes)
				}
				if err := m.uinput.KeyRelease(codes); err != nil {
					fmt.Println("Button release:", err)
				}
				delete(m.state.activeBinding, int(ev.Code))
			}
			if cancel, ok := m.state.activeMacroCancel[int(ev.Code)]; ok {
				cancel()
				delete(m.state.activeMacroCancel, int(ev.Code))
			}
		}
		m.state.buttonsHeld = heldButtons
	}

	if *debugMode {
		fmt.Println("---")
		for _, ev := range evs {
			fmt.Printf("TYPE: %d\tCODE: %d\tVALUE: %d\n", ev.Type, ev.Code, ev.Value)
		}
	}

	// TODO: Lock on configuration changes

	return
}

func slowJogTiming() time.Duration {
	conf := currentConfiguration
	if conf == nil {
		return 200 * time.Millisecond
	}
	slowJog := 200
	if conf.SlowJog != nil {
		slowJog = *conf.SlowJog
	}

	return time.Duration(slowJog) * time.Millisecond
}

func (m *Mapper) EmitOther(key string) error {
	conf := currentConfiguration
	if conf == nil {
		return fmt.Errorf("No configuration for this Window")
	}

	upperKey := strings.ToUpper(key)

	if *debugMode {
		fmt.Println("EmitOther:", key)
	}

	for _, binding := range conf.bindings {
		// A binding matches if its key is this movement (otherKey) or a plain
		// button (buttonDown) whose name equals this key.
		if binding.otherKey == upperKey || (binding.buttonDown != 0 && reverseShuttleKeys[binding.buttonDown] == upperKey) {
			return m.executeBindingTap(binding)
		}
	}

	return fmt.Errorf("No bindings for those movements")
}

// EmitOtherRepeat starts repeating a shuttle-position binding while the wheel
// stays held at that position. Unlike a plain tap, a held shuttle position is a
// continuous control: the command fires once immediately, then again every
// delayMS (default 25ms) until the wheel moves to another position or back to
// center (see dispatch) or the device is released (see ReleaseAll). The
// previous position's repeat, if any, is cancelled first.
func (m *Mapper) EmitOtherRepeat(key string) error {
	conf := currentConfiguration
	if conf == nil {
		return fmt.Errorf("No configuration for this Window")
	}

	upperKey := strings.ToUpper(key)

	if *debugMode {
		fmt.Println("EmitOtherRepeat:", key)
	}

	for _, binding := range conf.bindings {
		if binding.otherKey == upperKey {
			return m.startShuttleRepeat(binding)
		}
	}

	return fmt.Errorf("No bindings for those movements")
}

// startShuttleRepeat runs a binding's output as a repeated tap while the
// shuttle is held at its position. It cancels any repeat from a previous
// position, fires the command once immediately, then schedules a ticker that
// re-fires it every delayMS (default 25ms). The cancel func is stored in
// state.shuttleRepeat so dispatch (on position change / center) and ReleaseAll
// (on device loss / exit) can stop it.
func (m *Mapper) startShuttleRepeat(binding *deviceBinding) error {
	m.stopShuttleRepeat()

	codes, err := keyCodes(binding.original)
	if err != nil {
		return err
	}

	if *debugMode {
		fmt.Printf("uinput repeat %v every %dms\n", codes, binding.delayMS)
	}
	if *verboseMode {
		fmt.Printf("REPEAT key=%q output=%q codes=%v every %dms\n", binding.rawKey, binding.original, codes, binding.delayMS)
	}

	// Immediate first run, then repeat on a ticker.
	if err := m.uinput.KeyTap(codes); err != nil {
		return err
	}

	delay := binding.delayMS
	if delay <= 0 {
		delay = 25
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.state.shuttleRepeat = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(delay) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.uinput.KeyTap(codes); err != nil {
					fmt.Println("Shuttle repeat:", err)
					return
				}
			}
		}
	}()

	return nil
}

// stopShuttleRepeat cancels the active shuttle-repeat ticker, if any.
func (m *Mapper) stopShuttleRepeat() {
	if m.state.shuttleRepeat != nil {
		m.state.shuttleRepeat()
		m.state.shuttleRepeat = nil
	}
}

func (m *Mapper) EmitKeys(modifiers map[int]bool, keyDown int) ([]int, error) {
	conf := currentConfiguration
	if conf == nil {
		return nil, fmt.Errorf("No configuration for this Window")
	}

	if *debugMode {
		fmt.Println("Emit Keys", modifiers, reverseShuttleKeys[keyDown])
	}

	for _, binding := range conf.bindings {
		if reflect.DeepEqual(binding.heldButtons, modifiers) && binding.buttonDown == keyDown {
			return m.executeBinding(binding)
		}
	}

	return nil, fmt.Errorf("No binding for these keys: %s", describeKeyCombo(modifiers, keyDown))
}

// describeKeyCombo renders a button press (held modifiers + the pressed button)
// as a human-readable "M1+F2"-style string for use in error messages.
func describeKeyCombo(modifiers map[int]bool, keyDown int) string {
	parts := make([]string, 0, len(modifiers)+1)
	for code := range modifiers {
		parts = append(parts, reverseShuttleKeys[code])
	}
	parts = append(parts, reverseShuttleKeys[keyDown])
	sort.Strings(parts)
	return strings.Join(parts, "+")
}

func (m *Mapper) executeBinding(binding *deviceBinding) ([]int, error) {
	time.Sleep(25 * time.Millisecond)
	switch binding.driver {
	case "exec":
		if *debugMode {
			fmt.Printf("EXEC: /bin/bash -c %q\n", binding.original)
		}
		return nil, exec.Command("env", "bash", "-c", binding.original).Run()
	case "uinput":
		if len(binding.macros) > 0 {
			// Macro chains: run once (a one-shot) or repeat while held.
			// Either way nothing is held, so no key codes are returned.
			return nil, m.runMacroBinding(binding)
		}
		codes, err := keyCodes(binding.original)
		if err != nil {
			return nil, err
		}
		if *debugMode {
			fmt.Printf("uinput hold %v\n", codes)
		}
		if *verboseMode {
			fmt.Printf("HOLD key=%q output=%q codes=%v\n", binding.rawKey, binding.original, codes)
		}
		return codes, m.uinput.KeyHold(codes)
	case "osc":
		msgs := parseOSCMessages(binding.original)
		if msgs == nil {
			fmt.Printf("Failed parsing OSC binding for keys %q. Remember %q should start with an /\n", binding.rawKey, binding.rawValue)
			return nil, nil
		}
		for _, msg := range msgs {
			if msg.Address == "/sleep" {
				if *debugMode {
					fmt.Println("Sleeping for", msg.Arguments[0].(float64), "seconds")
				}
				time.Sleep(time.Duration(msg.Arguments[0].(float64)*1000) * time.Millisecond)
				continue
			}
			if *debugMode {
				fmt.Println("Sending OSC message:", msg)
			}
			err := binding.oscClient.Send(msg)
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	default:
		panic("unreachable")
	}
}

// tapKeys fires a single key (or key combo) binding.repeat times, sleeping
// binding.delayMS between taps and binding.startDelayMS before the first. The
// defaults (repeat 1, delay 25ms, no start delay) reproduce a plain tap, so
// ordinary bindings are unaffected.
func (m *Mapper) tapKeys(codes []int, binding *deviceBinding) error {
	if binding.startDelayMS > 0 {
		time.Sleep(time.Duration(binding.startDelayMS) * time.Millisecond)
	}
	for i := 0; i < binding.repeat; i++ {
		if i > 0 {
			time.Sleep(time.Duration(binding.delayMS) * time.Millisecond)
		}
		if err := m.uinput.KeyTap(codes); err != nil {
			return err
		}
	}
	return nil
}

// runMacroBinding runs a binding's macro chain on a button press. A one-shot
// chain (binding.once) runs exactly once. Any other chain runs immediately,
// then repeats every binding.delayMS (default 25ms) while the button stays
// held; a background goroutine drives the repeats and is cancelled on key-up
// (see dispatch) or on exit (see ReleaseAll).
func (m *Mapper) runMacroBinding(binding *deviceBinding) error {
	if binding.once {
		return m.executeMacroSequence(binding)
	}

	// Immediate first run.
	if err := m.executeMacroSequence(binding); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.state.activeMacroCancel[binding.buttonDown] = cancel

	delay := binding.delayMS
	if delay <= 0 {
		delay = 25
	}

	go func() {
		ticker := time.NewTicker(time.Duration(delay) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.executeMacroSequence(binding); err != nil {
					fmt.Println("Macro repeat:", err)
					return
				}
			}
		}
	}()

	return nil
}

// executeMacroSequence runs a binding's macro commands in order. Each command
// is a string starting with a "/" verb:
//
//	/type <text>            types <text> via the uinput device
//	/sleep <secs>           sleeps for <secs> seconds
//	/exec <cmd>             runs <cmd> through `bash -c`
//	/mousemove <x> <y> <abs>  moves the mouse; <abs> true makes the move absolute
//	/click <button> [<n>]  clicks <button> (a name like "left" or a hex code);
//	                          optional <n> repeats the click
//
// It stops at the first command that fails.
func (m *Mapper) executeMacroSequence(binding *deviceBinding) error {
	for _, macro := range binding.macros {
		fields := strings.Fields(macro)
		if len(fields) == 0 {
			return fmt.Errorf("empty macro in binding %q", binding.rawKey)
		}
		verb := fields[0]
		rest := strings.TrimSpace(macro[len(verb):])

		switch verb {
		case "/type":
			if rest == "" {
				return fmt.Errorf("/type in binding %q needs text to type", binding.rawKey)
			}
			if *debugMode {
				fmt.Printf("uinput type %q\n", rest)
			}
			if err := m.uinput.Type(rest); err != nil {
				return fmt.Errorf("uinput type: %s", err)
			}
		case "/sleep":
			if rest == "" {
				return fmt.Errorf("/sleep in binding %q needs a duration", binding.rawKey)
			}
			secs, err := strconv.ParseFloat(rest, 64)
			if err != nil {
				return fmt.Errorf("invalid /sleep duration %q: %s", rest, err)
			}
			if *debugMode {
				fmt.Println("Sleeping for", secs, "seconds")
			}
			time.Sleep(time.Duration(secs*1000) * time.Millisecond)
		case "/exec":
			if rest == "" {
				return fmt.Errorf("/exec in binding %q needs a command", binding.rawKey)
			}
			if *debugMode {
				fmt.Printf("EXEC: /bin/bash -c %q\n", rest)
			}
			if err := exec.Command("env", "bash", "-c", rest).Run(); err != nil {
				return fmt.Errorf("exec: %s", err)
			}
		case "/mousemove":
			args := strings.Fields(rest)
			if len(args) < 2 {
				return fmt.Errorf("/mousemove in binding %q needs x and y", binding.rawKey)
			}
			x, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid /mousemove x %q in binding %q: %s", args[0], binding.rawKey, err)
			}
			y, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid /mousemove y %q in binding %q: %s", args[1], binding.rawKey, err)
			}
			abs := false
			if len(args) >= 3 {
				abs, err = strconv.ParseBool(args[2])
				if err != nil {
					return fmt.Errorf("invalid /mousemove absolute flag %q: %s", args[2], err)
				}
			}
			if *debugMode {
				fmt.Printf("uinput mousemove %d %d abs=%v\n", x, y, abs)
			}
			if err := m.uinput.MouseMove(x, y, abs); err != nil {
				return fmt.Errorf("uinput mousemove: %s", err)
			}
		case "/click":
			args := strings.Fields(rest)
			if len(args) == 0 {
				return fmt.Errorf("/click in binding %q needs a button", binding.rawKey)
			}
			code, ok := mouseButtonCodes[strings.ToLower(args[0])]
			if !ok {
				return fmt.Errorf("unknown /click button %q in binding %q", args[0], binding.rawKey)
			}
			repeats := 1
			if len(args) >= 2 {
				n, err := strconv.Atoi(args[1])
				if err != nil || n < 1 {
					return fmt.Errorf("invalid /click repeat count %q in binding %q", args[1], binding.rawKey)
				}
				repeats = n
			}
			if *debugMode {
				fmt.Printf("uinput click %d x%d\n", code, repeats)
			}
			if err := m.uinput.Click(code, repeats); err != nil {
				return fmt.Errorf("uinput click: %s", err)
			}
		default:
			return fmt.Errorf("unknown macro %q in binding %q (use /type, /sleep, /exec, /mousemove, /click)", verb, binding.rawKey)
		}
	}
	return nil
}

func (m *Mapper) executeBindingTap(binding *deviceBinding) error {
	time.Sleep(25 * time.Millisecond)
	switch binding.driver {
	case "exec":
		if *debugMode {
			fmt.Printf("EXEC: /bin/bash -c %q\n", binding.original)
		}
		return exec.Command("env", "bash", "-c", binding.original).Run()
	case "uinput":
		if len(binding.macros) > 0 {
			// A macro chain: run it once (a one-shot) or repeat while held.
			// Nothing is held, so no key codes are returned.
			return m.runMacroBinding(binding)
		}
		codes, err := keyCodes(binding.original)
		if err != nil {
			return err
		}
		if *debugMode {
			fmt.Printf("uinput tap %v x%d\n", codes, binding.repeat)
		}
		return m.tapKeys(codes, binding)
	case "osc":
		msgs := parseOSCMessages(binding.original)
		if msgs == nil {
			fmt.Printf("Failed parsing OSC binding for keys %q. Remember %q should start with an /\n", binding.rawKey, binding.rawValue)
			return nil
		}
		for _, msg := range msgs {
			if msg.Address == "/sleep" {
				if *debugMode {
					fmt.Println("Sleeping for", msg.Arguments[0].(float64), "seconds")
				}
				time.Sleep(time.Duration(msg.Arguments[0].(float64)*1000) * time.Millisecond)
				continue
			}
			if *debugMode {
				fmt.Println("Sending OSC message:", msg)
			}
			err := binding.oscClient.Send(msg)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		panic("unreachable")
	}
}

func parseOSCMessages(multiInput string) (out []*osc.Message) {
	inputs := strings.Split(multiInput, " + ")
	for _, input := range inputs {
		msg := parseOSCMessage(strings.TrimSpace(input))
		if msg == nil {
			return nil
		}
		out = append(out, msg)
	}
	return
}

func parseOSCMessage(input string) *osc.Message {
	fields := strings.Fields(input) // move to something like `sh` interpretation (or quoted strings) if needed
	if len(fields) == 0 {
		return nil
	}

	if !strings.HasPrefix(fields[0], "/") {
		return nil
	}

	msg := osc.NewMessage(fields[0])
	for _, arg := range fields[1:] {
		if val, err := strconv.ParseFloat(arg, 64); err == nil {
			msg.Append(val)
		} else if val, err := strconv.ParseInt(arg, 10, 64); err == nil {
			msg.Append(val)
		} else if arg == "true" {
			msg.Append(true)
		} else if arg == "false" {
			msg.Append(false)
		} else if arg == "nil" {
			msg.Append(nil)
		} else if arg == "null" {
			msg.Append(nil)
		} else {
			msg.Append(arg)
		}
	}
	return msg
}

func jogVal(evs []evdev.InputEvent) int {
	for _, ev := range evs {
		if ev.Type == 2 && ev.Code == 7 {
			return int(ev.Value)
		}
	}
	return 0
}

func shuttleVal(evs []evdev.InputEvent) (out int) {
	for idx, ev := range evs {
		if ev.Type == 0 && idx != len(evs)-1 {
			out = 0
		}
		if ev.Type == 2 && ev.Code == 8 {
			out = int(ev.Value)
		}
	}
	return
}

func buttonVals(current map[int]bool, ev evdev.InputEvent) (out map[int]bool, lastDown int) {
	out = current

	if ev.Value == 1 {
		current[int(ev.Code)] = true
	} else {
		delete(current, int(ev.Code))
	}

	if ev.Value == 1 {
		lastDown = int(ev.Code)
	}

	return
}

func buttonsToModifiers(held map[int]bool, buttonDown int) (out map[int]bool) {
	out = make(map[int]bool)
	for k := range held {
		if k == buttonDown {
			continue
		}
		out[k] = true
	}
	return
}
