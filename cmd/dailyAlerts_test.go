package cmd

import (
	"strings"
	"testing"
	"time"

	"opslink/internal/incidentio"
	"opslink/internal/slack"
)

const sampleDatadogTitle = `[P1] [Triggered on {kube_cluster_name:cluster-staging,kube_deployment:staging-api,kube_namespace:payments}] [P1][PLATFORM][STAGING] Deployment staging-api com réplicas indisponíveis no cluster cluster-staging (namespace payments)`

func TestParseAlertTitleStripsTriggeredBlob(t *testing.T) {
	t.Parallel()

	parsed := parseAlertTitle(sampleDatadogTitle)
	if parsed.Severity != "P1" {
		t.Fatalf("severity = %q", parsed.Severity)
	}
	if !strings.HasPrefix(parsed.Title, "Deployment staging-api") {
		t.Fatalf("title = %q", parsed.Title)
	}
	if strings.Contains(parsed.Title, "Triggered") || strings.Contains(parsed.Title, "PLATFORM") {
		t.Fatalf("noise left in title: %q", parsed.Title)
	}
	if parsed.Labels["kube_deployment"] != "staging-api" {
		t.Fatalf("labels = %#v", parsed.Labels)
	}
	if parsed.Labels["kube_namespace"] != "payments" {
		t.Fatalf("namespace = %q", parsed.Labels["kube_namespace"])
	}
}

func TestParseAlertTitleReTriggered(t *testing.T) {
	t.Parallel()

	parsed := parseAlertTitle(`[P2] [Re-Triggered on {kube_cluster_name:prod,kube_namespace:app}] [STAGING] API latency`)
	if parsed.Severity != "P2" {
		t.Fatalf("severity = %q", parsed.Severity)
	}
	if parsed.Title != "API latency" {
		t.Fatalf("title = %q", parsed.Title)
	}
}

func TestFormatAlertLineShowsFullName(t *testing.T) {
	t.Parallel()

	line := formatAlertLine(incidentio.Alert{
		ID:        "01ALERT",
		SourceURL: "https://us5.datadoghq.com/event/event?id=123",
		Title:     sampleDatadogTitle,
		Attributes: []incidentio.AlertAttributeEntry{
			{
				Attribute: incidentio.AlertAttributeV2{Name: "Service"},
				Value: &incidentio.AlertAttributeValueV2{
					CatalogEntry: &struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					}{Name: "pgbouncer"},
				},
			},
		},
	})
	if strings.Contains(line, "…") || strings.Contains(line, "Triggered") {
		t.Fatalf("still truncated or noisy: %s", line)
	}
	if !strings.Contains(line, "réplicas indisponíveis") {
		t.Fatalf("missing human title: %s", line)
	}
	if !strings.Contains(line, "*P1*") {
		t.Fatalf("missing severity: %s", line)
	}
	if !strings.Contains(line, "pgbouncer") {
		t.Fatalf("missing service: %s", line)
	}
	if !strings.Contains(line, "us5.datadoghq.com") || !strings.Contains(line, "|:datadog: Datadog>") {
		t.Fatalf("missing Datadog permalink: %s", line)
	}
}

func TestAlertSourceLinkFallsBackToIncident(t *testing.T) {
	t.Parallel()

	href, label := alertSourceLink(incidentio.Alert{ID: "01ALERT"})
	if !strings.Contains(href, "01ALERT") || label != slack.IncidentLinkText {
		t.Fatalf("got %s %s", href, label)
	}
}

func TestBuildDailyAlertSectionsNoCodeFence(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	sections := buildDailyAlertSections([]dayAlerts{
		{
			date:  day,
			label: "Qua 12/08",
			alerts: []incidentio.Alert{
				{ID: "01A", Title: sampleDatadogTitle},
			},
		},
	})
	if len(sections) != 1 {
		t.Fatalf("sections = %d", len(sections))
	}
	if strings.Contains(sections[0], "```") {
		t.Fatalf("should not use a code block: %s", sections[0])
	}
	if !strings.Contains(sections[0], "*Qua 12/08*") {
		t.Fatalf("missing day header: %s", sections[0])
	}
}
