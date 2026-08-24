package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDuplicateBindingKeys guards the duplicate-key detection: a bindings
// object that repeats a key (e.g. two "JogL") must be reported, because Go's
// map silently keeps only one value per key and would drop the rest.
func TestDuplicateBindingKeys(t *testing.T) {
	cases := []struct {
		name     string
		bindings string
		want     []string
	}{
		{
			name:     "no duplicates",
			bindings: `{"F1":"a","JogL":"left","JogR":"right"}`,
			want:     nil,
		},
		{
			name:     "duplicate JogL",
			bindings: `{"JogL":"left","JogR":"right","JogL":"a"}`,
			want:     []string{"JogL"},
		},
		{
			name:     "two distinct duplicates",
			bindings: `{"F1":"a","F1":"b","JogL":"left","JogL":"x"}`,
			want:     []string{"F1", "JogL"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := duplicateBindingKeys(json.RawMessage(c.bindings))
			if !equalStr(got, c.want) {
				t.Errorf("duplicateBindingKeys(%s) = %v, want %v", c.bindings, got, c.want)
			}
		})
	}
}

// TestCheckDuplicateBindingsEndToEnd runs the full LoadConfig path against a
// temp file with a duplicate key and asserts the error names the key.
func TestCheckDuplicateBindingsEndToEnd(t *testing.T) {
	cfg := `{
		"apps": [
			{
				"name": "Global",
				"match_window_titles": [".*"],
				"bindings": {
					"JogL": "left",
					"JogR": "right",
					"JogL": "a"
				}
			}
		]
	}`
	f := filepath.Join(t.TempDir(), "dup.json")
	if err := os.WriteFile(f, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	err := LoadConfig(f)
	if err == nil {
		t.Fatal("expected an error for a duplicate binding key, got nil")
	}
	if !strings.Contains(err.Error(), "JogL") {
		t.Errorf("error should name the duplicate key JogL, got: %s", err)
	}
}
