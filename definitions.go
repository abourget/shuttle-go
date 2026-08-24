package main

import (
	"fmt"
	"strconv"
	"strings"
)

var shuttleKeys = map[string]int{
	"F1": 256,
	"F2": 257,
	"F3": 258,
	"F4": 259,
	"F5": 260,
	"F6": 261,
	"F7": 262,
	"F8": 263,
	"F9": 264,
	"B1": 267,
	"B2": 265,
	"B3": 266,
	"B4": 268,
	"M1": 269,
	"M2": 270,
	"M3": 272,
	"M4": 273,
}

// mToM1M2 remaps the M3/M4 button codes (272/273, as reported by some Shuttle
// Pro variants) onto the M1/M2 codes (269/270). The two variants never report
// both at once, so a config written with M1/M2 fires identically for either.
func mToM1M2(code int) int {
	switch code {
	case 272:
		return 269
	case 273:
		return 270
	}
	return code
}

var otherShuttleKeys = map[string]bool{
	"S-7":      true,
	"S-6":      true,
	"S-5":      true,
	"S-4":      true,
	"S-3":      true,
	"S-2":      true,
	"S-1":      true,
	"S0":       true,
	"S1":       true,
	"S2":       true,
	"S3":       true,
	"S4":       true,
	"S5":       true,
	"S6":       true,
	"S7":       true,
	"JogL":     true,
	"JogR":     true,
	"SlowJogL": true,
	"SlowJogR": true,
}

var keyboardKeys = map[string]int{
	"Esc":        1,
	"1":          2,
	"2":          3,
	"3":          4,
	"4":          5,
	"5":          6,
	"6":          7,
	"7":          8,
	"8":          9,
	"9":          10,
	"0":          11,
	"Minus":      12,
	"-":          12,
	"Equal":      13,
	"=":          13,
	"Backspace":  14,
	"Tab":        15,
	"Q":          16,
	"W":          17,
	"E":          18,
	"R":          19,
	"T":          20,
	"Y":          21,
	"U":          22,
	"I":          23,
	"O":          24,
	"P":          25,
	"LeftBrace":  26,
	"RightBrace": 27,
	"{":          26,
	"}":          27,
	"Enter":      28,
	"LeftCtrl":   29,
	"Ctrl":       29,
	"A":          30,
	"S":          31,
	"D":          32,
	"F":          33,
	"G":          34,
	"H":          35,
	"J":          36,
	"K":          37,
	"L":          38,
	"Semicolon":  39,
	";":          39,
	"Apostrophe": 40,
	"'":          40,
	"Grave":      41,
	"`":          41,
	"LeftShift":  42,
	"Shift":      42,
	"Backslash":  43,
	"\\":         43,
	"Z":          44,

	// Raw symbol keys. These are the shifted forms of the base keys above
	// (e.g. "5" shifted is "%", "7" shifted is "&"), so their keycodes match
	// the base key and a binding writes them as "Shift+<symbol>". The named
	// aliases for these (Percent, Ampersand, ...) were removed in favor of the
	// literal symbol, which is more readable in a config.
	"!":          11, // 0
	"@":          2,  // 1
	"#":          3,  // 2
	"$":          4,  // 3
	"%":          5,  // 4
	"^":          6,  // 5
	"&":          7,  // 6
	"*":          8,  // 7
	"(":          9,  // 8
	")":          10, // 9
	"_":          12, // -
	"+":          13, // =
	"|":          43, // \
	":":          39, // ;
	"<":          51, // ,
	">":          52, // .
	"?":          53, // /
	"~":          41, // `
	"X":          45,
	"C":          46,
	"V":          47,
	"B":          48,
	"N":          49,
	"M":          50,
	"Comma":      51,
	",":          51,
	"Dot":        52,
	".":          52,
	"Slash":      53,
	"/":          53,
	"RightShift": 54,
	"RShift":     54,
	"KPAsterisk": 55,
	"LeftAlt":    56,
	"Alt":        56,
	"Space":      57,
	"CapsLock":   58,
	"F1":         59,
	"F2":         60,
	"F3":         61,
	"F4":         62,
	"F5":         63,
	"F6":         64,
	"F7":         65,
	"F8":         66,
	"F9":         67,
	"F10":        68,
	"NumLock":    69,
	"ScrollLock": 70,
	"KP7":        71,
	"KP8":        72,
	"KP9":        73,
	"KPMinus":    74,
	"KP4":        75,
	"KP5":        76,
	"KP6":        77,
	"KPPlus":     78,
	"KP1":        79,
	"KP2":        80,
	"KP3":        81,
	"KP0":        82,
	"KPDot":      83,
	"F11":        87,
	"F12":        88,

	"Henkan": 92,

	"KPEnter":         96,
	"RightCtrl":       97,
	"RCtrl":           97,
	"RightAlt":        100,
	"RAlt":            100,
	"Linefeed":        101,
	"Home":            102,
	"Up":              103,
	"PageUp":          104,
	"PgUp":            104,
	"Left":            105,
	"Right":           106,
	"End":             107,
	"Down":            108,
	"PageDown":        109,
	"PgDown":          109,
	"PgDn":            109,
	"Insert":          110,
	"Delete":          111,
	"Macro":           112,
	"Mute":            113,
	"VolumeDown":      114,
	"VolumeUp":        115,
	"Power":           116, /*ScSystemPowerDown*/
	"KPEqual":         117,
	"KPPlusMinus":     118,
	"Pause":           119,
	"Scale":           120, /*AlCompizScale(Expose)*/
	"KPComma":         121,
	"LeftMeta":        125,
	"Meta":            125,
	"RightMeta":       126,
	"RMeta":           126,
	"Compose":         127,
	"Stop":            128, /*AcStop*/
	"Again":           129,
	"Props":           130, /*AcProperties*/
	"Undo":            131, /*AcUndo*/
	"Front":           132,
	"Copy":            133, /*AcCopy*/
	"Open":            134, /*AcOpen*/
	"Paste":           135, /*AcPaste*/
	"Find":            136, /*AcSearch*/
	"Cut":             137, /*AcCut*/
	"Help":            138, /*AlIntegratedHelpCenter*/
	"Menu":            139, /*Menu(ShowMenu)*/
	"Calc":            140, /*AlCalculator*/
	"Setup":           141,
	"Sleep":           142, /*ScSystemSleep*/
	"Wakeup":          143, /*SystemWakeUp*/
	"File":            144, /*AlLocalMachineBrowser*/
	"SendFile":        145,
	"DeleteFile":      146,
	"Xfer":            147,
	"Prog1":           148,
	"Prog2":           149,
	"WWW":             150, /*AlInternetBrowser*/
	"Coffee":          152, /*AlTerminalLock/Screensaver*/
	"Direction":       153,
	"CycleWindows":    154,
	"Mail":            155,
	"Bookmarks":       156, /*AcBookmarks*/
	"Computer":        157,
	"Back":            158, /*AcBack*/
	"Forward":         159, /*AcForward*/
	"CloseCD":         160,
	"EjectCD":         161,
	"EjectCloseCD":    162,
	"NextSong":        163,
	"PlayPause":       164,
	"PreviousSong":    165,
	"StopCD":          166,
	"Record":          167,
	"Rewind":          168,
	"Phone":           169, /*MediaSelectTelephone*/
	"ISO":             170,
	"Config":          171, /*AlConsumerControlConfiguration*/
	"Homepage":        172, /*AcHome*/
	"Refresh":         173, /*AcRefresh*/
	"Exit":            174, /*AcExit*/
	"Move":            175,
	"Edit":            176,
	"ScrollUp":        177,
	"ScrollDown":      178,
	"KPLeftParen":     179,
	"KPRightParen":    180,
	"New":             181, /*AcNew*/
	"Redo":            182, /*AcRedo/Repeat*/
	"F13":             183,
	"F14":             184,
	"F15":             185,
	"F16":             186,
	"F17":             187,
	"F18":             188,
	"F19":             189,
	"F20":             190,
	"F21":             191,
	"F22":             192,
	"F23":             193,
	"F24":             194,
	"PlayCD":          200,
	"PauseCD":         201,
	"Prog3":           202,
	"Prog4":           203,
	"Dashboard":       204, /*AlDashboard*/
	"Suspend":         205,
	"Close":           206, /*AcClose*/
	"Play":            207,
	"FastForward":     208,
	"Print":           210, /*AcPrint*/
	"Camera":          212,
	"Sound":           213,
	"Question":        214,
	"Email":           215,
	"Chat":            216,
	"Search":          217,
	"Connect":         218,
	"Finance":         219, /*AlCheckbook/Finance*/
	"Sport":           220,
	"Shop":            221,
	"AltErase":        222,
	"Cancel":          223, /*AcCancel*/
	"BrightnessDown":  224,
	"BrightnessUp":    225,
	"Media":           226,
	"Send":            231, /*AcSend*/
	"Reply":           232, /*AcReply*/
	"ForwardMail":     233, /*AcForwardMsg*/
	"Save":            234, /*AcSave*/
	"Documents":       235,
	"BrightnessCycle": 243, /*BrightnessUp,AfterMaxIsMin*/
	"BrightnessZero":  244, /*BrightnessOff,UseAmbient*/
	"DisplayOff":      245, /*DisplayDeviceToOffState*/
	"Rfkill":          247, /*KeyThatControlsAllRadios*/
	"Micmute":         248, /*Mute/UnmuteTheMicrophone*/
}

var reverseShuttleKeys = map[int]string{}
var keyboardKeysUpper = map[string]int{}
var otherShuttleKeysUpper = map[string]bool{}

func init() {
	for k, v := range shuttleKeys {
		reverseShuttleKeys[v] = k
	}
	for k, v := range keyboardKeys {
		keyboardKeysUpper[strings.ToUpper(k)] = v
	}
	for k, v := range otherShuttleKeys {
		otherShuttleKeysUpper[strings.ToUpper(k)] = v
	}
}

// keyCode translates a single key name to a Linux input keycode.
// Accepts either a hex keycode ("0xff51") or a name from keyboardKeys ("S", "Enter", ...).
func keyCode(key string) (int, error) {
	if strings.HasPrefix(key, "0x") {
		code, err := strconv.ParseInt(key, 0, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid hex keycode %q: %s", key, err)
		}
		return int(code), nil
	}
	if code, ok := keyboardKeysUpper[strings.ToUpper(key)]; ok {
		return code, nil
	}
	return 0, fmt.Errorf("unknown key %q (use a keyboardKeys name or a 0x keycode)", key)
}

// keyCodes translates a config value that may contain "+"-separated keys
// (e.g. "Shift+Tab") into a slice of Linux input keycodes, preserving order.
func keyCodes(value string) ([]int, error) {
	parts := strings.Split(value, "+")
	codes := make([]int, len(parts))
	for i, part := range parts {
		code, err := keyCode(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

// runeKeyCode maps a rune to the EV_KEY code that types it, and whether Shift
// must be held to produce that character. It is used by the /type macro to emit
// text. Letters and unshifted symbols map directly; shifted symbols (e.g. '!',
// '@', '~') map to their base key with shift required. Space and the common
// printable ASCII are all covered.
func runeKeyCode(r rune) (code int, shift bool, err error) {
	if m, ok := runeKeyMap[r]; ok {
		return m.code, m.shift, nil
	}
	return 0, false, fmt.Errorf("no keycode for rune %q", r)
}

type runeKey struct {
	code  int
	shift bool
}

// runeKeyMap covers printable US-QWERTY ASCII. Letters/digits/space map to
// their own key; symbols that need Shift map to the base key with shift set.
var runeKeyMap = map[rune]runeKey{
	'a': {30, false}, 'b': {31, false}, 'c': {46, false}, 'd': {32, false},
	'e': {18, false}, 'f': {33, false}, 'g': {34, false}, 'h': {35, false},
	'i': {23, false}, 'j': {36, false}, 'k': {37, false}, 'l': {38, false},
	'm': {50, false}, 'n': {49, false}, 'o': {24, false}, 'p': {25, false},
	'q': {16, false}, 'r': {19, false}, 's': {31, false}, 't': {20, false},
	'u': {22, false}, 'v': {47, false}, 'w': {17, false}, 'x': {45, false},
	'y': {21, false}, 'z': {44, false},
	'A': {30, true}, 'B': {31, true}, 'C': {46, true}, 'D': {32, true},
	'E': {18, true}, 'F': {33, true}, 'G': {34, true}, 'H': {35, true},
	'I': {23, true}, 'J': {36, true}, 'K': {37, true}, 'L': {38, true},
	'M': {50, true}, 'N': {49, true}, 'O': {24, true}, 'P': {25, true},
	'Q': {16, true}, 'R': {19, true}, 'S': {31, true}, 'T': {20, true},
	'U': {22, true}, 'V': {47, true}, 'W': {17, true}, 'X': {45, true},
	'Y': {21, true}, 'Z': {44, true},
	'1': {2, false}, '2': {3, false}, '3': {4, false}, '4': {5, false},
	'5': {6, false}, '6': {7, false}, '7': {8, false}, '8': {9, false},
	'9': {10, false}, '0': {11, false},
	'!': {11, true}, '@': {2, true}, '#': {3, true}, '$': {4, true},
	'%': {5, true}, '^': {6, true}, '&': {7, true}, '*': {8, true},
	'(': {9, true}, ')': {10, true},
	'-': {12, false}, '_': {12, true},
	'=': {13, false}, '+': {13, true},
	'[': {26, false}, '{': {26, true},
	']': {27, false}, '}': {27, true},
	'\\': {43, false}, '|': {43, true},
	';': {39, false}, ':': {39, true},
	'\'': {40, false}, '"': {40, true},
	',': {51, false}, '<': {51, true},
	'.': {52, false}, '>': {52, true},
	'/': {53, false}, '?': {53, true},
	'`': {41, false}, '~': {41, true},
	' ': {57, false},
}
