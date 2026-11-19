package oncall

import "testing"

func TestBuildSlackGroupMemberIDs(t *testing.T) {
	t.Parallel()

	got := BuildSlackGroupMemberIDs(
		[]string{"U_PRIMARY", "U_SECONDARY"},
		[]string{"U_EXTRA"},
	)
	want := []string{"U_PRIMARY", "U_SECONDARY", "U_EXTRA"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestPlanSlackGroupOmitsEmptySecondary(t *testing.T) {
	t.Parallel()

	got := PlanSlackGroup("U_PRIMARY", "", []string{"U_EXTRA"})
	if len(got) != 2 || got[0] != "U_PRIMARY" || got[1] != "U_EXTRA" {
		t.Fatalf("members = %v", got)
	}

	withSecondary := PlanSlackGroup("U_PRIMARY", "U_SECONDARY", nil)
	if len(withSecondary) != 2 || withSecondary[0] != "U_PRIMARY" || withSecondary[1] != "U_SECONDARY" {
		t.Fatalf("members with secondary = %v", withSecondary)
	}
}
