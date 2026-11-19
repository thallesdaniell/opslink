package slack

import "testing"

func TestIncidentFooter(t *testing.T) {
	t.Parallel()

	got := IncidentFooter("Ver todos", "https://example.com", IncidentLinkText)
	want := "*:incidentio: Ver todos:* <https://example.com|Incident.io>"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestAssembleMessageOrder(t *testing.T) {
	t.Parallel()

	blocks := AssembleMessage(HeaderDailyAlerts, []string{"dia 1", "dia 2"}, "footer")
	if len(blocks) != 6 {
		t.Fatalf("len = %d want 6 (header, s1, divider, s2, divider, footer)", len(blocks))
	}
	if blocks[0].BlockType != "header" || blocks[0].Text != HeaderDailyAlerts {
		t.Fatalf("header = %+v", blocks[0])
	}
	if blocks[1].BlockType != "section" || blocks[2].BlockType != "divider" {
		t.Fatalf("body = %+v %+v", blocks[1], blocks[2])
	}
	if blocks[5].BlockType != "context" || blocks[5].Text != "footer" {
		t.Fatalf("footer = %+v", blocks[5])
	}
}

func TestComposeFooterPutsIncidentLinkFirst(t *testing.T) {
	t.Parallel()

	got := ComposeFooter("Ver todos", "https://example.com", "_3 alerta(s)_")
	want := "*:incidentio: Ver todos:* <https://example.com|Incident.io>\n_3 alerta(s)_"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestComposeLinkFooterSkipsBlank(t *testing.T) {
	t.Parallel()

	got := ComposeLinkFooter(":cloud:", "Comunicações oficiais", GCPStatusURL, "GCP Status", "", "_12 emails_")
	want := "*:cloud: Comunicações oficiais:* <https://status.cloud.google.com|GCP Status>\n_12 emails_"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestAssembleMessageSkipsEmptySections(t *testing.T) {
	t.Parallel()

	blocks := AssembleMessage(HeaderNextOncall, []string{"", "dia 1"}, "footer")
	if len(blocks) != 4 {
		t.Fatalf("len = %d want 4 (header, section, divider, footer)", len(blocks))
	}
	if blocks[1].Text != "dia 1" {
		t.Fatalf("first real section = %+v", blocks[1])
	}
}
