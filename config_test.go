package main

import (
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
