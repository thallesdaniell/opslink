package cmd

import (
	"strings"
	"testing"
	"time"

	"opslink/internal/gemini"
	"opslink/internal/slack"
)

func TestBuildWeeklyGcpSectionsSkipsEmptyAndListsActions(t *testing.T) {
	t.Parallel()

	sections := buildWeeklyGcpSections(&gemini.Report{
		Critico: []gemini.Item{{
			Title:   "Permissão no sink `bigquery-table-schema-changed`",
			Detail:  "perda de logs",
			Product: "Logging",
			Link:    "https://status.cloud.google.com/incidents/abc",
		}},
		Acoes: []gemini.Item{{
			Title: "Conceder pubsub.publisher à SA do sink",
		}},
		Resolvidos: 5,
	})
	if len(sections) != 2 {
		t.Fatalf("sections = %d want 2, got %#v", len(sections), sections)
	}
	if strings.Contains(sections[0], "Relevante") || strings.Contains(strings.Join(sections, "\n"), "foi resolvido") {
		t.Fatalf("should omit empty/informational dump: %#v", sections)
	}
	if !strings.Contains(sections[0], "*:red_circle: Crítico*") || !strings.Contains(sections[0], "<https://status.cloud.google.com/incidents/abc|status>") {
		t.Fatalf("critical section: %s", sections[0])
	}
	if strings.Contains(sections[0], ":red_circle:  Permissão") {
		t.Fatalf("icon should be title-only: %s", sections[0])
	}
	if !strings.Contains(sections[0], "• Permissão no sink") {
		t.Fatalf("missing bullet: %s", sections[0])
	}
	if !strings.Contains(sections[0], "_Logging · perda de logs_") {
		t.Fatalf("missing meta: %s", sections[0])
	}
	if !strings.Contains(sections[1], "*:hammer_and_wrench: Ações*") {
		t.Fatalf("actions: %s", sections[1])
	}
}

func TestBuildWeeklyGcpSectionsResolvedOnly(t *testing.T) {
	t.Parallel()

	sections := buildWeeklyGcpSections(&gemini.Report{Resolvidos: 8})
	if len(sections) != 1 || sections[0] != ":large_green_circle:  _Sem pendências operacionais nesta semana._" {
		t.Fatalf("got %#v", sections)
	}
}

func TestBuildWeeklyGcpSectionsEmpty(t *testing.T) {
	t.Parallel()

	sections := buildWeeklyGcpSections(&gemini.Report{})
	if len(sections) != 1 || sections[0] != emptyWeeklyGcpSection {
		t.Fatalf("got %#v", sections)
	}
}

func TestSafeHTTPURLRejectsJavascript(t *testing.T) {
	t.Parallel()

	if safeHTTPURL("javascript:alert(1)") != "" {
		t.Fatal("javascript url should be rejected")
	}
	if safeHTTPURL("https://console.cloud.google.com/iam") == "" {
		t.Fatal("https url should pass")
	}
}

func TestWeeklyGcpFooter(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	got := weeklyGcpFooter(12, start, end, 5)
	if !strings.Contains(got, "*:gcp: Comunicações oficiais:* <"+slack.GCPStatusURL+"|GCP Status>") {
		t.Fatalf("missing status link: %s", got)
	}
	if !strings.Contains(got, "*⚠️ 12 email(s) · 10/08 → 17/08 · 5 já resolvido(s).*") {
		t.Fatalf("meta: %s", got)
	}
}

func TestGcpLinkLabel(t *testing.T) {
	t.Parallel()

	if gcpLinkLabel("https://status.cloud.google.com/incidents/x") != "status" {
		t.Fatal("status")
	}
	if gcpLinkLabel("https://mail.google.com/mail/u/0/#all/t") != "email" {
		t.Fatal("email")
	}
}
