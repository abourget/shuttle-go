package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBindingAndDescription(t *testing.T) {
	tests := []struct {
		in         string
		bind, desc string
	}{
		{"Ctrl+A", "Ctrl+A", ""},
		{"Ctrl+A // ", "Ctrl+A", ""},
		{"Ctrl+A  // Description", "Ctrl+A", "Description"},
		{"Ctrl+A//Description", "Ctrl+A", "Description"},
		{"Ctrl+A    //    Description", "Ctrl+A", "Description"},
	}

	for idx, test := range tests {
		bind, desc := bindingAndDescription("xdo", test.in)
		assert.Equal(t, test.bind, bind, "%d", idx)
		assert.Equal(t, test.desc, desc, "%d", idx)
	}
}

// TestDecodeBindingValue covers the three value forms: a plain string, a
// macro array, and the object form with its per-binding knobs. It also checks
// that defaults are applied for unset knobs.
func TestDecodeBindingValue(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bindingValue
	}{
		{
			name: "plain string",
			raw:  `"a"`,
			want: bindingValue{Key: "a", Repeat: 1, DelayMS: 25},
		},
		{
			name: "macro array",
			raw:  `["/type hi","/sleep 1"]`,
			want: bindingValue{Repeat: 1, DelayMS: 25, Macros: []string{"/type hi", "/sleep 1"}},
		},
		{
			name: "object with knobs",
			raw:  `{"key":"s","repeat":3,"delay_ms":100,"start_delay_ms":5,"once":true}`,
			want: bindingValue{Key: "s", Repeat: 3, DelayMS: 100, StartDelayMS: 5, Once: true},
		},
		{
			name: "object defaults",
			raw:  `{"key":"a"}`,
			want: bindingValue{Key: "a", Repeat: 1, DelayMS: 25},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := decodeBindingValue(json.RawMessage(c.raw))
			assert.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}
