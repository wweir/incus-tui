package client

import (
	"strings"
	"testing"
)

func TestSanitizeOutput(t *testing.T) {
	short := "  hello\n"
	if got := sanitizeOutput(short); got != "hello" {
		t.Fatalf("sanitizeOutput short = %q", got)
	}

	long := strings.Repeat("a", 2000)
	got := sanitizeOutput(long)
	if len(got) != 1027 {
		t.Fatalf("sanitizeOutput long len = %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("sanitizeOutput long suffix = %q", got[len(got)-3:])
	}
}
