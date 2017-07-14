package main

import "strings"

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
	"LeftShift":  42,
	"Shift":      42,
	"Backslash":  43,
	"\\":         43,
	"Z":          44,
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
	"*":          55,
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
	"(":               179,
	"KPRightParen":    180,
	")":               180,
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
