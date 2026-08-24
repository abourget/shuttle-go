package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	evdev "github.com/gvalkov/golang-evdev"
)

// rawBindings converts a map of binding values (string or []string) into the
// map[string]json.RawMessage shape AppConfig.Bindings expects.
func rawBindings(t *testing.T, in map[string]interface{}) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(in))
	for k, v := range in {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshaling binding %q: %s", k, err)
		}
		out[k] = json.RawMessage(b)
	}
	return out
}

// fakeUinput records the events a uinputDevice would emit, so the binding
// execution paths (mapper.go) can be unit-tested without a real /dev/uinput.
// It mirrors the *uinputDevice method set used by the mapper.
type fakeUinput struct {
	taps     [][]int
	holds    [][]int
	releases [][]int
	typed    []string
	moves    []string
	clicks   []string
}

func (f *fakeUinput) KeyTap(codes []int) error {
	cp := append([]int(nil), codes...)
	f.taps = append(f.taps, cp)
	return nil
}

func (f *fakeUinput) KeyHold(codes []int) error {
	cp := append([]int(nil), codes...)
	f.holds = append(f.holds, cp)
	return nil
}

func (f *fakeUinput) KeyRelease(codes []int) error {
	cp := append([]int(nil), codes...)
	f.releases = append(f.releases, cp)
	return nil
}

func (f *fakeUinput) Type(text string) error {
	f.typed = append(f.typed, text)
	return nil
}

func (f *fakeUinput) MouseMove(dx, dy int, abs bool) error {
	f.moves = append(f.moves, fmt.Sprintf("%d,%d,abs=%v", dx, dy, abs))
	return nil
}

func (f *fakeUinput) Click(code int, repeats int) error {
	f.clicks = append(f.clicks, fmt.Sprintf("%d x%d", code, repeats))
	return nil
}

func (f *fakeUinput) Destroy() {}

// codesOf joins a recorded key-code slice into a "1,2,3" string for assertions.
func codesOf(codes []int) string {
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = fmt.Sprintf("%d", c)
	}
	return strings.Join(parts, ",")
}

// testConfig builds a parsed AppConfig (uinput driver) from a bindings map.
// The app matches every window so it is always the active configuration.
func testConfig(t *testing.T, bindings map[string]interface{}) *AppConfig {
	return testConfigWithSlowJog(t, bindings, nil)
}

// testConfigWithSlowJog is testConfig with an explicit slow_jog value (nil =
// unset, i.e. the 200ms default).
func testConfigWithSlowJog(t *testing.T, bindings map[string]interface{}, slowJog *int) *AppConfig {
	t.Helper()
	conf := &Config{Apps: []*AppConfig{{
		Name:              "test",
		MatchWindowTitles: []string{".*"},
		Driver:            "uinput",
		SlowJog:           slowJog,
		Bindings:          rawBindings(t, bindings),
	}}}
	if err := conf.Apps[0].parse(); err != nil {
		t.Fatalf("parse: %s", err)
	}
	if err := conf.Apps[0].parseBindings(); err != nil {
		t.Fatalf("parseBindings: %s", err)
	}
	return conf.Apps[0]
}

// testMapper builds a Mapper whose uinput is a fresh fake and whose active
// configuration is the one built by testConfig.
func testMapper(t *testing.T, bindings map[string]interface{}) (*Mapper, *fakeUinput) {
	t.Helper()
	conf := testConfig(t, bindings)
	currentConfiguration = conf
	fake := &fakeUinput{}
	m := NewMapper(nil, nil)
	m.uinput = fake // uinputDevice and fakeUinput share the same method set
	return m, fake
}

// TestEveryBinding exercises every binding kind the mapper can produce:
// plain key taps, modifier+key holds, shuttle taps/holds, jog taps, and
// macro chains. It asserts each one resolves to the expected uinput event,
// which is the class of bug that produces "No bindings for those movements".
func TestEveryBinding(t *testing.T) {
	keybindings := map[string]interface{}{
		// Plain single-key taps (button press).
		"F1": "a",
		"F2": "s",
		"F3": "d",
		"F4": "f",
		"F5": "g",
		"F6": "h",
		"F7": "j",
		"F8": "k",
		"F9": "l",
		"B1": "q",
		"B2": "w",
		"B3": "e",
		"B4": "r",
		"M1": "t",
		"M2": "y",

		// Modifier + key holds (held buttons, then the pressed button).
		"M1+F1": "Ctrl+A",
		"M2+F2": "Shift+Tab",
		"B1+B2+F3": "Ctrl+Shift+Space",

		// Shuttle taps (S0) and holds (S-7..S7).
		"S0":  "Space",
		"S-7": "Up",
		"S-6": "Up",
		"S-5": "Up",
		"S-4": "Up",
		"S-3": "Up",
		"S-2": "Up",
		"S-1": "Up",
		"S1":  "Down",
		"S2":  "Down",
		"S3":  "Down",
		"S4":  "Down",
		"S5":  "Down",
		"S6":  "Down",
		"S7":  "Down",

		// Jog taps.
		"JogL":     "Left",
		"JogR":     "Right",
		"SlowJogL": "Home",
		"SlowJogR": "End",
	}

	m, fake := testMapper(t, keybindings)

	// --- Plain key taps: a lone button press resolves to a KeyTap. ---
	tapCases := []struct {
		key  string
		want string
	}{
		{"F1", "30"}, {"F2", "31"}, {"F3", "32"}, {"F4", "33"}, {"F5", "34"},
		{"F6", "35"}, {"F7", "36"}, {"F8", "37"}, {"F9", "38"},
		{"B1", "16"}, {"B2", "17"}, {"B3", "18"}, {"B4", "19"},
		{"M1", "20"}, {"M2", "21"},
	}
	for _, c := range tapCases {
		if err := m.executeBindingTap(m.confBinding(c.key)); err != nil {
			t.Fatalf("executeBindingTap(%q) error: %s", c.key, err)
		}
		if got := codesOf(fake.taps[len(fake.taps)-1]); got != c.want {
			t.Errorf("executeBindingTap(%q): tap = %s, want %s", c.key, got, c.want)
		}
	}

	// --- Modifier + key holds: a held modifier plus a pressed button
	// resolves to a KeyHold with the combined codes (modifier first, then key).
	holdCases := []struct {
		key  string
		want string
	}{
		{"M1+F1", "29,30"},      // Ctrl, A
		{"M2+F2", "42,15"},      // Shift, Tab
		{"B1+B2+F3", "29,42,57"}, // Ctrl, Shift, Space
	}
	for _, c := range holdCases {
		parts := strings.Split(c.key, "+")
		down := shuttleKeys[strings.ToUpper(parts[len(parts)-1])]
		modifiers := map[int]bool{}
		for _, p := range parts[:len(parts)-1] {
			modifiers[shuttleKeys[strings.ToUpper(p)]] = true
		}
		if _, err := m.EmitKeys(modifiers, down); err != nil {
			t.Fatalf("EmitKeys(%q) error: %s", c.key, err)
		}
		if got := codesOf(fake.holds[len(fake.holds)-1]); got != c.want {
			t.Errorf("EmitKeys(%q): hold = %s, want %s", c.key, got, c.want)
		}
	}

	// --- Shuttle tap (S0) and holds (S-7..S7). ---
	if err := m.EmitOther("S0"); err != nil {
		t.Fatalf("EmitOther(S0) error: %s", err)
	}
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "57" {
		t.Errorf("EmitOther(S0): tap = %s, want 57", got)
	}

	// Held shuttle positions repeat their command while held; the first fire is
	// an immediate tap, so each position should record a tap of its key.
	repeats := []struct {
		key  string
		want string
	}{
		{"S-7", "103"}, {"S-6", "103"}, {"S-5", "103"}, {"S-4", "103"},
		{"S-3", "103"}, {"S-2", "103"}, {"S-1", "103"},
		{"S1", "108"}, {"S2", "108"}, {"S3", "108"}, {"S4", "108"},
		{"S5", "108"}, {"S6", "108"}, {"S7", "108"},
	}
	for _, c := range repeats {
		if err := m.EmitOtherRepeat(c.key); err != nil {
			t.Fatalf("EmitOtherRepeat(%q) error: %s", c.key, err)
		}
		if got := codesOf(fake.taps[len(fake.taps)-1]); got != c.want {
			t.Errorf("EmitOtherRepeat(%q): first tap = %s, want %s", c.key, got, c.want)
		}
	}
	// Stop the last position's repeat ticker so its goroutine does not keep
	// tapping (and racing the assertions below) after the loop.
	m.stopShuttleRepeat()

	// --- Jog taps. ---
	for key, want := range map[string]string{
		"JogL": "105", "JogR": "106", "SlowJogL": "102", "SlowJogR": "107",
	} {
		if err := m.EmitOther(key); err != nil {
			t.Fatalf("EmitOther(%q) error: %s", key, err)
		}
		if got := codesOf(fake.taps[len(fake.taps)-1]); got != want {
			t.Errorf("EmitOther(%q): tap = %s, want %s", key, got, want)
		}
	}
}

// TestMacroBindings exercises the macro-chain execution paths (the uinput
// driver with a JSON-array value): /type, /sleep, /exec, /mousemove, /click,
// and the one-shot "once" flag.
func TestMacroBindings(t *testing.T) {
	macros := map[string]interface{}{
		"F1": []string{
			"/type hello",
			"/sleep 0",
			"/exec true",
			"/mousemove 10 20",
			"/mousemove 5 5 true",
			"/click left",
			"/click right 3",
		},
		"F2": map[string]interface{}{
			"once":   true,
			"macros": []string{"/type once", "/click middle"},
		},
	}

	m, fake := testMapper(t, macros)

	// F1: a repeating chain. Run the sequence directly (executeBindingTap
	// would spawn a repeat goroutine for a non-one-shot chain).
	if err := m.executeMacroSequence(m.confBinding("F1")); err != nil {
		t.Fatalf("executeMacroSequence(F1) error: %s", err)
	}
	if got, want := fake.typed, []string{"hello"}; !equalStr(got, want) {
		t.Errorf("F1 /type = %v, want %v", got, want)
	}
	if got, want := fake.moves, []string{"10,20,abs=false", "5,5,abs=true"}; !equalStr(got, want) {
		t.Errorf("F1 /mousemove = %v, want %v", got, want)
	}
	if got, want := fake.clicks, []string{"272 x1", "273 x3"}; !equalStr(got, want) {
		t.Errorf("F1 /click = %v, want %v", got, want)
	}

	// F2: a one-shot chain (object form with "once": true).
	if err := m.executeMacroSequence(m.confBinding("F2")); err != nil {
		t.Fatalf("executeMacroSequence(F2) error: %s", err)
	}
	if got, want := fake.typed, []string{"hello", "once"}; !equalStr(got, want) {
		t.Errorf("typed after F2 = %v, want %v", got, want)
	}
	if got, want := fake.clicks, []string{"272 x1", "273 x3", "274 x1"}; !equalStr(got, want) {
		t.Errorf("clicks after F2 = %v, want %v", got, want)
	}
}

// TestRepeatTapBinding exercises the object-form binding knobs: a tap binding
// with repeat/delay fires the key that many times, and the defaults (a plain
// string) fire exactly once.
func TestRepeatTapBinding(t *testing.T) {
	bindings := map[string]interface{}{
		"F1": "a",
		"F2": map[string]interface{}{
			"key":      "s",
			"repeat":   3,
			"delay_ms":   1,
		},
	}
	m, fake := testMapper(t, bindings)

	// F1: a plain string -> a single tap.
	if err := m.executeBindingTap(m.confBinding("F1")); err != nil {
		t.Fatalf("executeBindingTap(F1) error: %s", err)
	}
	if got := len(fake.taps); got != 1 {
		t.Errorf("F1 taps = %d, want 1", got)
	}

	// F2: repeat 3 -> three taps of the same code.
	if err := m.executeBindingTap(m.confBinding("F2")); err != nil {
		t.Fatalf("executeBindingTap(F2) error: %s", err)
	}
	if got := len(fake.taps); got != 4 { // 1 (F1) + 3 (F2)
		t.Fatalf("total taps = %d, want 4", got)
	}
	for i, want := range []string{"31", "31", "31"} {
		if got := codesOf(fake.taps[1+i]); got != want {
			t.Errorf("F2 tap %d = %s, want %s", i, got, want)
		}
	}
}

// TestJogDispatchRegression guards the jog-movement name contract: the config
// keys are the spaced "JogL"/"JogR"/"SlowJogL"/"SlowJogR" (what otherKey is
// set to at parse time), and the dispatch must emit exactly those names so the
// lookup resolves. A mismatched name (e.g. the old spaced "Jog R", or the
// unspaced "JogR" the dispatch briefly emitted) would fail the lookup.
func TestJogDispatchRegression(t *testing.T) {
	m, fake := testMapper(t, map[string]interface{}{
		"JogL":     "left",
		"JogR":     "right",
		"SlowJogL": "home",
		"SlowJogR": "end",
	})

	// A right jog: delta +1, not slow -> "JogR" -> right (106).
	m.state.jog = 0
	m.state.lastJog = time.Now()
	m.dispatch([]evdev.InputEvent{{Type: 2, Code: 7, Value: 1}})
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "106" {
		t.Errorf("jog right: tap = %s, want 106 (Right)", got)
	}

	// A left jog: delta -1 -> "JogL" -> left (105).
	m.state.jog = 1
	m.dispatch([]evdev.InputEvent{{Type: 2, Code: 7, Value: 0}})
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "105" {
		t.Errorf("jog left: tap = %s, want 105 (Left)", got)
	}

	// A slow jog (lastJog in the distant past), right direction ->
	// "SlowJogR" -> end (107).
	m.state.jog = 0
	m.state.lastJog = time.Now().Add(-time.Second)
	m.dispatch([]evdev.InputEvent{{Type: 2, Code: 7, Value: 1}})
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "107" {
		t.Errorf("slow jog right: tap = %s, want 107 (End)", got)
	}

	// A slow jog, left direction -> "SlowJogL" -> home (102).
	m.state.jog = 1
	m.state.lastJog = time.Now().Add(-time.Second)
	m.dispatch([]evdev.InputEvent{{Type: 2, Code: 7, Value: 0}})
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "102" {
		t.Errorf("slow jog left: tap = %s, want 102 (Home)", got)
	}
}

// TestJogDisabledSlowJog guards the slow_jog=0 case: when slow jog is
// disabled, a jog must NOT be classified as slow (the old code compared
// time.Since(lastJog) against a 0ms threshold, which is always true, so every
// jog became "SlowJog*" and never matched a plain JogL/JogR binding).
func TestJogDisabledSlowJog(t *testing.T) {
	zero := 0
	conf := testConfigWithSlowJog(t, map[string]interface{}{
		"JogL": "left",
		"JogR": "right",
	}, &zero)
	currentConfiguration = conf
	fake := &fakeUinput{}
	m := NewMapper(nil, nil)
	m.uinput = fake

	// A right jog, with the previous jog a long time ago. With slow_jog=0
	// this must still be a normal "JogR" (right, 106), not "SlowJogR".
	m.state.jog = 0
	m.state.lastJog = time.Now().Add(-time.Hour)
	m.dispatch([]evdev.InputEvent{{Type: 2, Code: 7, Value: 1}})
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "106" {
		t.Errorf("jog right with slow_jog=0: tap = %s, want 106 (Right); got slow-classified instead", got)
	}

	// A left jog, likewise.
	m.state.jog = 1
	m.state.lastJog = time.Now().Add(-time.Hour)
	m.dispatch([]evdev.InputEvent{{Type: 2, Code: 7, Value: 0}})
	if got := codesOf(fake.taps[len(fake.taps)-1]); got != "105" {
		t.Errorf("jog left with slow_jog=0: tap = %s, want 105 (Left); got slow-classified instead", got)
	}
}

// TestM3M4DispatchRegression guards the M3/M4 -> M1/M2 translation: a device
// that reports the M1/M2 buttons as 272/273 must fire the M1/M2 bindings, and
// the release must clear the same translated code.
func TestM3M4DispatchRegression(t *testing.T) {
	m, fake := testMapper(t, map[string]interface{}{
		"M1": "t",
		"M2": "y",
	})

	// Press M3 (272) -> treated as M1 (269) -> hold "t" (20).
	m.dispatch([]evdev.InputEvent{{Type: 1, Code: 272, Value: 1}})
	if got := codesOf(fake.holds[len(fake.holds)-1]); got != "20" {
		t.Errorf("M3 press: hold = %s, want 20 (t)", got)
	}
	// The held binding is tracked under the translated code (269, not 272).
	if _, ok := m.state.activeBinding[269]; !ok {
		t.Errorf("activeBinding should track M3 under 269 (M1)")
	}
	if _, ok := m.state.activeBinding[272]; ok {
		t.Errorf("activeBinding should not track M3 under its raw code 272")
	}

	// Release M3 (272) -> release tracked under 269.
	m.dispatch([]evdev.InputEvent{{Type: 1, Code: 272, Value: 0}})
	if got := codesOf(fake.releases[len(fake.releases)-1]); got != "20" {
		t.Errorf("M3 release: release = %s, want 20 (t)", got)
	}

	// Press M4 (273) -> treated as M2 (270) -> hold "y" (21).
	m.dispatch([]evdev.InputEvent{{Type: 1, Code: 273, Value: 1}})
	if got := codesOf(fake.holds[len(fake.holds)-1]); got != "21" {
		t.Errorf("M4 press: hold = %s, want 21 (y)", got)
	}
	m.dispatch([]evdev.InputEvent{{Type: 1, Code: 273, Value: 0}})
	if got := codesOf(fake.releases[len(fake.releases)-1]); got != "21" {
		t.Errorf("M4 release: release = %s, want 21 (y)", got)
	}
}

// confBinding returns the parsed deviceBinding for a raw key from the active
// configuration, for direct use in macro tests.
func (m *Mapper) confBinding(key string) *deviceBinding {
	for _, b := range currentConfiguration.bindings {
		if b.rawKey == key {
			return b
		}
	}
	return nil
}

func equalStr(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
