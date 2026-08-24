package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConfigFile writes content to a fresh temp file and returns its path.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shuttle-go.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigWatcherNoChangeIsNoOp(t *testing.T) {
	path := writeConfigFile(t, `{"apps":[{"name":"A","bindings":{}}]}`)
	if err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	before := loadedConfiguration

	cw := NewConfigWatcher(path, nil)
	cw.check()

	if loadedConfiguration != before {
		t.Fatal("expected no reload when the config file is unchanged")
	}
}

func TestConfigWatcherReloadsOnChange(t *testing.T) {
	path := writeConfigFile(t, `{"apps":[{"name":"A","bindings":{}}]}`)
	if err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	before := loadedConfiguration

	// Construct the watcher against the as-loaded file (as the real flow does),
	// then edit the file and force a newer mtime so the change is detected even
	// on coarse clocks.
	cw := NewConfigWatcher(path, nil)

	future := time.Now().Add(time.Minute)
	if err := os.WriteFile(path, []byte(`{"apps":[{"name":"B","bindings":{}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now(), future); err != nil {
		t.Fatal(err)
	}

	cw.check()

	if loadedConfiguration == before {
		t.Fatal("expected a reload after the config file changed")
	}
	if len(loadedConfiguration.Apps) != 1 || loadedConfiguration.Apps[0].Name != "B" {
		t.Fatalf("expected reloaded app B, got %+v", loadedConfiguration.Apps)
	}
}

func TestConfigWatcherKeepsPreviousOnInvalidEdit(t *testing.T) {
	path := writeConfigFile(t, `{"apps":[{"name":"A","bindings":{}}]}`)
	if err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	before := loadedConfiguration

	cw := NewConfigWatcher(path, nil)

	future := time.Now().Add(time.Minute)
	if err := os.WriteFile(path, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now(), future); err != nil {
		t.Fatal(err)
	}

	cw.check()

	if loadedConfiguration != before {
		t.Fatal("expected the previous config to be kept after an invalid edit")
	}
}

// saveTempRename mimics an editor that saves by writing a temp file in the same
// directory and renaming it over the target, replacing the target's inode.
func saveTempRename(t *testing.T, target, content string) {
	t.Helper()
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".shuttle-go.tmp-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		t.Fatal(err)
	}
}

// waitForApp polls until the loaded config's first app has the given name,
// failing on timeout.
func waitForApp(t *testing.T, want string, done chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if loadedConfiguration != nil && len(loadedConfiguration.Apps) == 1 &&
			loadedConfiguration.Apps[0].Name == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for app %q to be loaded", want)
		}
		select {
		case <-done:
			t.Fatal("watcher loop exited unexpectedly")
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestConfigWatcherReloadsAcrossRenameSaves drives the real Run() (fsnotify
// watching the parent directory) through two consecutive temp+rename saves, each
// of which replaces the file's inode, and asserts a reload happens for both.
// This is the regression test for "only the first config change is caught".
func TestConfigWatcherReloadsAcrossRenameSaves(t *testing.T) {
	path := writeConfigFile(t, `{"apps":[{"name":"A","bindings":{}}]}`)
	if err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		NewConfigWatcher(path, nil).Run()
		close(done)
	}()
	// Give the watcher goroutine a moment to reach its read loop before the
	// first save, so its early events are not dropped.
	time.Sleep(300 * time.Millisecond)

	saveTempRename(t, path, `{"apps":[{"name":"B","bindings":{}}]}`)
	waitForApp(t, "B", done)

	saveTempRename(t, path, `{"apps":[{"name":"C","bindings":{}}]}`)
	waitForApp(t, "C", done)
}
