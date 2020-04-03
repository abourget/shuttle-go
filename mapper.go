package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	evdev "github.com/gvalkov/golang-evdev"
	"github.com/hypebeast/go-osc/osc"
)

// Mapper receives events from the Shuttle devices, and maps (through
// configuration) to the Virtual Keyboard events.
type Mapper struct {
	inputDevice *evdev.InputDevice
	state       buttonsState
	watcher     *watcher
}

type buttonsState struct {
	jog         int
	shuttle     int
	buttonsHeld map[int]bool
	lastJog     time.Time
}

func NewMapper(inputDevice *evdev.InputDevice) *Mapper {
	m := &Mapper{
		inputDevice: inputDevice,
	}
	m.state.buttonsHeld = make(map[int]bool)
	m.state.jog = -1
	return m
}

func (m *Mapper) Process() error {
	evs, err := m.inputDevice.Read()
	if err != nil {
		return err
	}

	m.dispatch(evs)

	return nil
}

func (m *Mapper) dispatch(evs []evdev.InputEvent) {
	newJogVal := jogVal(evs)
	if m.state.jog != newJogVal {
		if m.state.jog != -1 {
			if m.state.lastJog.IsZero() {
				m.state.lastJog = time.Now()
			}

			slow := ""
			if time.Since(m.state.lastJog) > slowJogTiming() {
				slow = "Slow"
			}
			// Trigger JL or JR if we're advancing or not..
			delta := newJogVal - m.state.jog
			if (delta > 0 || delta < -200) && (delta < 200) {
				if err := m.EmitOther(slow + "JogR"); err != nil {
					fmt.Println("Jog right:", err)
				}
			} else {
				if err := m.EmitOther(slow + "JogL"); err != nil {
					fmt.Println("Jog left:", err)
				}
			}

			m.state.lastJog = time.Now()
		}
		m.state.jog = newJogVal
	}

	newShuttleVal := shuttleVal(evs)
	if m.state.shuttle != newShuttleVal {
		keyName := fmt.Sprintf("S%d", newShuttleVal)
		if *debugMode {
			fmt.Println("SHUTTLE", keyName)
		}
		if err := m.EmitOther(keyName); err != nil {
			fmt.Printf("Shuttle movement %q: %s\n", keyName, err)
		}
		m.state.shuttle = newShuttleVal
	}

	for _, ev := range evs {
		if ev.Type != 1 {
			continue
		}

		heldButtons, lastDown := buttonVals(m.state.buttonsHeld, ev)
		if lastDown != 0 {
			modifiers := buttonsToModifiers(heldButtons, lastDown)
			if err := m.EmitKeys(modifiers, lastDown); err != nil {
				fmt.Println("Button press:", err)
			}
			// fmt.Printf("OUTPUT: Modifiers: %v,  Just pressed: %d\n", modifiers, lastDown)
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
		if binding.otherKey == upperKey {
			return m.executeBinding(binding)
		}
	}

	return fmt.Errorf("No bindings for those movements")
}

func (m *Mapper) EmitKeys(modifiers map[int]bool, keyDown int) error {
	conf := currentConfiguration
	if conf == nil {
		return fmt.Errorf("No configuration for this Window")
	}

	if *debugMode {
		fmt.Println("Emit Keys", modifiers, reverseShuttleKeys[keyDown])
	}

	for _, binding := range conf.bindings {
		if reflect.DeepEqual(binding.heldButtons, modifiers) && binding.buttonDown == keyDown {
			return m.executeBinding(binding)
		}
	}

	return fmt.Errorf("No binding for these keys")
}

func (m *Mapper) executeBinding(binding *deviceBinding) error {
	time.Sleep(25 * time.Millisecond)
	switch binding.driver {
	case "exec":
		fmt.Printf("EXEC: /bin/bash -c %q\n", binding.original)
		return exec.Command("/bin/bash", "-c", binding.original).Run()
	case "xdotool", "":
		fmt.Println("xdotool key --clearmodifiers", binding.original)
		return exec.Command("xdotool", "key", "--clearmodifiers", binding.original).Run()
	case "osc":
		msgs := parseOSCMessages(binding.original)
		if msgs == nil {
			fmt.Printf("Failed parsing OSC binding for keys %q. Remember %q should start with an /\n", binding.rawKey, binding.rawValue)
			return nil
		}
		for _, msg := range msgs {
			if msg.Address == "/sleep" {
				fmt.Println("Sleeping for", msg.Arguments[0].(float64), "seconds")
				time.Sleep(time.Duration(msg.Arguments[0].(float64)*1000) * time.Millisecond)
				continue
			}
			fmt.Println("Sending OSC message:", msg)
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
