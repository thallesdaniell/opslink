package cmd

import "testing"

func TestFormatOncallRoleLinesOmitsEmptySecondary(t *testing.T) {
	t.Parallel()

	got := formatOncallRoleLines(formatOncallMention("U_P"), "")
	if got != "Primário: <@U_P>\n" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatOncallDaySection(t *testing.T) {
	t.Parallel()

	got := formatOncallDaySection("Seg 17/08", "<@U_P>", "<@U_S>")
	want := "*Seg 17/08*\nPrimário: <@U_P>\nSecundário: <@U_S>"
	if got != want {
		t.Fatalf("got %q", got)
	}

	primaryOnly := formatOncallDaySection("Sáb 22/08", "<@U_P>", "")
	if primaryOnly != "*Sáb 22/08*\nPrimário: <@U_P>" {
		t.Fatalf("primary only: %q", primaryOnly)
	}
	if formatOncallDaySection("Dom 23/08", "", "") != "" {
		t.Fatal("empty day should be omitted")
	}
}
