package gmail

import (
	"strings"
	"testing"
)

func TestExtractURLsDedupAndTrim(t *testing.T) {
	t.Parallel()

	text := "veja https://status.cloud.google.com/incidents/abc, e de novo https://status.cloud.google.com/incidents/abc."
	got := extractURLs(text)
	if len(got) != 1 || got[0] != "https://status.cloud.google.com/incidents/abc" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseRowsSkipsEmptyAndExtractsLinks(t *testing.T) {
	t.Parallel()

	got := ParseRows([][]interface{}{
		{},
		{"2026-08-17", "GCP incident", "Google Cloud", "Mon", "falha https://status.cloud.google.com/incidents/abc", ""},
		{"2026-08-17", "", "", "", "", ""},
	})
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Subject != "GCP incident" {
		t.Fatalf("subject = %q", got[0].Subject)
	}
	if len(got[0].Links) != 1 || !strings.Contains(got[0].Links[0], "status.cloud.google.com") {
		t.Fatalf("links = %#v", got[0].Links)
	}
}

func TestFormatDigest(t *testing.T) {
	t.Parallel()

	got := FormatDigest([]Message{{
		Subject: "GCP incident",
		From:    "Google",
		Date:    "Mon",
		Body:    "falha no GKE",
		Links:   []string{"https://status.cloud.google.com/incidents/abc"},
	}})
	if !strings.Contains(got, "ASSUNTO: GCP incident") || !strings.Contains(got, "LINKS:") {
		t.Fatalf("got %q", got)
	}
}
