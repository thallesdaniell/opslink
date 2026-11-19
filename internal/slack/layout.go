package slack

import (
	"fmt"
	"strings"
)

const (
	IncidentSchedulesURL = "https://app.incident.io/on-call/schedules"
	IncidentLinkText     = "Incident.io"
	GCPStatusURL         = "https://status.cloud.google.com"

	HeaderDailyAlerts  = "Alertas firing"
	HeaderOncallUpdate = "Plantão atualizado"
	HeaderNextOncall   = "Rotação semanal"
	HeaderWeeklyGcp    = "Resumo GCP / K8s"
)

// LinkFooter is the shared context line: *:emoji: action:* <url|label>
func LinkFooter(emoji, action, url, linkText string) string {
	if emoji == "" {
		emoji = ":incidentio:"
	}
	if linkText == "" {
		linkText = IncidentLinkText
	}
	return fmt.Sprintf("*%s %s:* <%s|%s>", emoji, action, url, linkText)
}

// IncidentFooter is the shared context line for Incident.io links.
// Example: *:incidentio: Ver todos:* <url|Incident.io>
func IncidentFooter(action, url, linkText string) string {
	return LinkFooter(":incidentio:", action, url, linkText)
}

// ComposeFooter puts the Incident.io link first, then optional extra context lines.
func ComposeFooter(action, url string, extra ...string) string {
	return ComposeLinkFooter(":incidentio:", action, url, IncidentLinkText, extra...)
}

// ComposeLinkFooter is ComposeFooter with a custom emoji and link label.
func ComposeLinkFooter(emoji, action, url, linkText string, extra ...string) string {
	parts := []string{LinkFooter(emoji, action, url, linkText)}
	for _, line := range extra {
		if line == "" {
			continue
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n")
}

// AssembleMessage builds the standard Opslink Slack layout:
// header → section(s) with dividers → divider → context footer.
func AssembleMessage(header string, sections []string, footer string) []MySlackBlock {
	parts := []MySlackBlock{
		CreateSectionBlock(header, "header", "plain_text"),
	}

	first := true
	for _, section := range sections {
		if section == "" {
			continue
		}
		if !first {
			parts = append(parts, CreateSectionBlock("", "divider", ""))
		}
		first = false
		parts = append(parts, CreateSectionBlock(section, "section", "mrkdwn"))
	}

	if footer != "" {
		parts = append(parts, CreateSectionBlock("", "divider", ""))
		parts = append(parts, CreateSectionBlock(footer, "context", "mrkdwn"))
	}

	return parts
}
