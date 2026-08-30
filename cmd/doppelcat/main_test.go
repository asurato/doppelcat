package main

import "testing"

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
