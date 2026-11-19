package gemini

import (
	"strings"
	"testing"
)

func TestParseReportFromFencedJSON(t *testing.T) {
	t.Parallel()

	raw := "```json\n{\"critico\":[\"Permissão no sink de Logging\"],\"relevante\":[],\"acoes\":[{\"titulo\":\"Conceder pubsub.publisher\",\"link\":\"https://console.cloud.google.com\"}],\"resolvidos\":5}\n```"
	report, err := ParseReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Critico) != 1 || report.Critico[0].Title != "Permissão no sink de Logging" {
		t.Fatalf("critico = %#v", report.Critico)
	}
	if report.Acoes[0].Link != "https://console.cloud.google.com" {
		t.Fatalf("link = %q", report.Acoes[0].Link)
	}
	if report.Resolvidos != 5 {
		t.Fatalf("resolvidos = %d", report.Resolvidos)
	}
}

func TestParseReportCapsAndDedup(t *testing.T) {
	t.Parallel()

	items := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		items = append(items, `"evento `+string(rune('A'+i))+`"`)
	}
	raw := `{"critico":[` + strings.Join(items, ",") + `,"evento A"],"relevante":[],"acoes":[],"resolvidos":0}`
	report, err := ParseReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Critico) != maxCritico {
		t.Fatalf("len = %d want %d (%#v)", len(report.Critico), maxCritico, report.Critico)
	}
}

func TestExtractJSONObject(t *testing.T) {
	t.Parallel()

	got, err := extractJSONObject("prefix {\"a\":1} suffix")
	if err != nil || got != `{"a":1}` {
		t.Fatalf("got %q err %v", got, err)
	}
}
