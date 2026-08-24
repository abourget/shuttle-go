package main

import "testing"

func TestUsesOSCDriver(t *testing.T) {
	// No config loaded: no OSC.
	loadedConfiguration = nil
	if usesOSCDriver() {
		t.Fatal("expected no OSC driver when no config is loaded")
	}

	// uinput-only config: no OSC.
	loadedConfiguration = &Config{Apps: []*AppConfig{
		{Name: "Global", bindings: []*deviceBinding{{driver: "uinput"}}},
	}}
	if usesOSCDriver() {
		t.Fatal("expected no OSC driver for a uinput-only config")
	}

	// One osc binding among several: OSC present.
	loadedConfiguration = &Config{Apps: []*AppConfig{
		{Name: "Global", bindings: []*deviceBinding{{driver: "uinput"}, {driver: "osc"}}},
	}}
	if !usesOSCDriver() {
		t.Fatal("expected OSC driver to be detected")
	}

	loadedConfiguration = nil
}
