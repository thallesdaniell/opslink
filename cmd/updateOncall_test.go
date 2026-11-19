package cmd

import (
	"strings"
	"testing"

	"opslink/internal/incidentio"
	"opslink/internal/slack"
)

const (
	testPrimaryEmail   = "primary@example.com"
	testSecondaryEmail = "secondary@example.com"
	testPairEmail      = "pair@example.com"
	testPairSlackID    = "U_PAIR"
)

func TestOncallUserFromScheduleSecondaryOptional(t *testing.T) {
	t.Parallel()

	user, ok, err := oncallUserFromSchedule("secondary", &incidentio.Schedule{
		Name:          "SRE-secundário",
		CurrentShifts: nil,
	})
	if err != nil {
		t.Fatalf("secondary empty should not error: %v", err)
	}
	if ok {
		t.Fatalf("secondary empty should omit user, got %+v", user)
	}
}

func TestOncallUserFromSchedulePrimaryRequired(t *testing.T) {
	t.Parallel()

	_, ok, err := oncallUserFromSchedule("primary", &incidentio.Schedule{
		Name:          "SRE-primário",
		CurrentShifts: nil,
	})
	if err == nil {
		t.Fatal("primary empty should error")
	}
	if ok {
		t.Fatal("primary empty should not yield a user")
	}
	if !strings.Contains(err.Error(), "SRE-primário") {
		t.Fatalf("error should name the schedule, got %v", err)
	}
}

func TestOncallUserFromSchedulePopulated(t *testing.T) {
	t.Parallel()

	user, ok, err := oncallUserFromSchedule("secondary", &incidentio.Schedule{
		Name: "SRE-secundário",
		CurrentShifts: []incidentio.CurrentShifts{
			{User: incidentio.User{
				Email:       testPairEmail,
				Name:        "Pair",
				SlackUserID: testPairSlackID,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected secondary user")
	}
	if user.Email != testPairEmail || user.SlackUserID != testPairSlackID {
		t.Fatalf("got %+v", user)
	}
}

func TestPreviousOncallFooter(t *testing.T) {
	t.Parallel()

	both := previousOncallFooter("U_P", "U_S")
	if !strings.Contains(both, "<@U_P> e <@U_S>") {
		t.Fatalf("both: %s", both)
	}
	incidentIdx := strings.Index(both, "*:incidentio:")
	thanksIdx := strings.Index(both, "Obrigado")
	if incidentIdx < 0 || thanksIdx < incidentIdx {
		t.Fatalf("Incident.io link should come before thanks: %s", both)
	}

	onlyPrimary := previousOncallFooter("U_P", "")
	if !strings.Contains(onlyPrimary, "<@U_P>!") {
		t.Fatalf("primary only: %s", onlyPrimary)
	}
	if strings.Contains(onlyPrimary, "e <@") {
		t.Fatalf("primary only should not thank a second person: %s", onlyPrimary)
	}

	none := previousOncallFooter("", "")
	if strings.Contains(none, "Obrigado") {
		t.Fatalf("empty previous should skip thanks: %s", none)
	}
	if !strings.HasPrefix(none, "*:incidentio:") || !strings.Contains(none, slack.IncidentSchedulesURL) {
		t.Fatalf("footer should start with shared Incident.io link: %s", none)
	}
}

func TestOncallWouldChangePrimaryOnly(t *testing.T) {
	t.Parallel()

	current := map[string]OncallUser{
		"primary": {Email: testPrimaryEmail},
	}

	if !oncallWouldChange(current, []string{
		testPrimaryEmail,
		testSecondaryEmail,
	}) {
		t.Fatal("dropping secondary from GCP should count as a change")
	}

	if oncallWouldChange(current, []string{testPrimaryEmail}) {
		t.Fatal("primary-only matching GCP should be in sync")
	}
}

func TestResolvePreviousOncallSlackIDsOverrides(t *testing.T) {
	t.Parallel()

	primary, secondary, err := resolvePreviousOncallSlackIDs(nil, nil, "U_P", "")
	if err != nil {
		t.Fatal(err)
	}
	if primary != "U_P" || secondary != "" {
		t.Fatalf("got %s %s", primary, secondary)
	}

	primary, secondary, err = resolvePreviousOncallSlackIDs(nil, []string{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if primary != "" || secondary != "" {
		t.Fatalf("empty GCP should skip previous ids, got %s %s", primary, secondary)
	}
}
