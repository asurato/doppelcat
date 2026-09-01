package main

import (
	"testing"
	"time"
)

func TestArgumentExitCodes(t *testing.T) {
	if got := run([]string{"--version"}); got != 0 {
		t.Fatalf("version=%d", got)
	}
	if got := run(nil); got == 0 {
		t.Fatal("missing argument accepted")
	}
	if got := run([]string{"definitely-not-present"}); got == 0 {
		t.Fatal("missing file accepted")
	}
}

func TestAppVersionUsesBuildFlag(t *testing.T) {
	previous := version
	version = "v1.2.3"
	t.Cleanup(func() { version = previous })

	if got := appVersion(); got != "v1.2.3" {
		t.Fatalf("appVersion() = %q", got)
	}
}

func TestParseRunArgsUpdateDelay(t *testing.T) {
	path, debounce, err := parseRunArgs([]string{"--update-delay", "750", "test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "test.txt" {
		t.Fatalf("path = %q", path)
	}
	if debounce != 750*time.Millisecond {
		t.Fatalf("debounce = %v", debounce)
	}
}

func TestParseRunArgsRejectsInvalidUpdateDelay(t *testing.T) {
	for _, value := range []string{"invalid", "500ms", "1.5", "0", "-1"} {
		if _, _, err := parseRunArgs([]string{"--update-delay", value, "test.txt"}); err == nil {
			t.Errorf("--update-delay %q accepted", value)
		}
	}
}
