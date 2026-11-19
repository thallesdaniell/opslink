package incidentio

import "testing"

func TestCurrentShiftEmail(t *testing.T) {
	t.Parallel()

	if got := currentShiftEmail(nil); got != "(sem plantão atual)" {
		t.Fatalf("nil shifts = %q", got)
	}

	if got := currentShiftEmail([]CurrentShifts{}); got != "(sem plantão atual)" {
		t.Fatalf("empty shifts = %q", got)
	}

	got := currentShiftEmail([]CurrentShifts{
		{User: User{Email: "sre@example.com"}},
	})
	if got != "sre@example.com" {
		t.Fatalf("populated shifts = %q", got)
	}
}
