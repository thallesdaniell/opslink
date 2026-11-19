package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestBuildNextOncallSectionsOmitsEmptySecondary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	dayMap := map[string]*oncallDayEntry{
		"2026-08-17": {Date: "Seg 17/08", Primary: "<@U_P>", Secondary: "<@U_S>"},
		"2026-08-22": {Date: "Sáb 22/08", Primary: "<@U_P>"},
	}

	sections := buildNextOncallSections(now, dayMap)
	if len(sections) != 2 {
		t.Fatalf("sections = %d want 2, got %#v", len(sections), sections)
	}
	if strings.Contains(sections[0], "```") || strings.Contains(sections[1], "```") {
		t.Fatalf("should not use a code table: %#v", sections)
	}
	if !strings.Contains(sections[1], "Primário: <@U_P>") {
		t.Fatalf("weekend primary: %s", sections[1])
	}
	if strings.Contains(sections[1], "Secundário") {
		t.Fatalf("weekend should omit empty secondary: %s", sections[1])
	}
}
