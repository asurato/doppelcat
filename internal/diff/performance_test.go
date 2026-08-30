package diff

import (
	"strings"
	"testing"
	"time"
)

func TestTenMBUpdateWithinSecond(t *testing.T) {
	line := strings.Repeat("x", 99) + "\n"
	oldText := strings.Repeat(line, 105000)
	newText := "changed\n" + oldText[len(line):]
	started := time.Now()
	got, _ := Lines(oldText, newText)
	if len(got) == 0 {
		t.Fatal("empty diff")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("diff took %v", elapsed)
	}
}
