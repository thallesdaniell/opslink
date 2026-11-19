package cmd

import (
	"fmt"
	"opslink/internal/environments"
	"opslink/internal/incidentio"
	"opslink/internal/slack"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	reTriggeredBlob     = regexp.MustCompile(`(?i)\s*\[(?:Re-)?Triggered on \{[^}]*\}\]\s*`)
	reTriggeredLabels   = regexp.MustCompile(`(?i)\[(?:Re-)?Triggered on \{([^}]*)\}\]`)
	reLeadingBracketTag = regexp.MustCompile(`(?i)^\[([^]]+)\]\s*`)
	reSeverityTag       = regexp.MustCompile(`(?i)\[(P\d+)\]`)
)

const (
	alertPermalinkFmt       = "https://app.incident.io/alerts/%s"
	slackSectionMax         = 2900
	emptyDailyAlertsSection = "_Nenhum alerta firing nos times monitorados._"
)

var dailyAlertsDryRun bool

type dayAlerts struct {
	date   time.Time
	label  string
	alerts []incidentio.Alert
}

type parsedAlertTitle struct {
	Severity string
	Title    string
	Labels   map[string]string
}

func parseAlertTitle(raw string) parsedAlertTitle {
	parsed := parsedAlertTitle{
		Labels: extractTriggeredLabels(raw),
	}

	if m := reSeverityTag.FindStringSubmatch(raw); len(m) == 2 {
		parsed.Severity = strings.ToUpper(m[1])
	}

	cleaned := reTriggeredBlob.ReplaceAllString(raw, " ")
	for {
		next := reLeadingBracketTag.ReplaceAllString(cleaned, "")
		if next == cleaned {
			break
		}
		cleaned = next
	}
	parsed.Title = strings.Join(strings.Fields(cleaned), " ")
	return parsed
}

func extractTriggeredLabels(raw string) map[string]string {
	labels := make(map[string]string)
	match := reTriggeredLabels.FindStringSubmatch(raw)
	if len(match) < 2 {
		return labels
	}
	for _, part := range strings.Split(match[1], ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			labels[key] = value
		}
	}
	return labels
}

func alertAttributeValue(alert incidentio.Alert, name string) string {
	for _, entry := range alert.Attributes {
		if !strings.EqualFold(entry.Attribute.Name, name) {
			continue
		}
		if entry.Value != nil && entry.Value.CatalogEntry != nil && entry.Value.CatalogEntry.Name != "" {
			return entry.Value.CatalogEntry.Name
		}
		if entry.Value != nil && entry.Value.Literal != "" {
			return entry.Value.Literal
		}
		for _, value := range entry.ArrayValue {
			if value.CatalogEntry != nil && value.CatalogEntry.Name != "" {
				return value.CatalogEntry.Name
			}
			if value.Literal != "" {
				return value.Literal
			}
		}
	}
	return ""
}

func alertMetaLine(alert incidentio.Alert, parsed parsedAlertTitle) string {
	parts := make([]string, 0, 4)
	add := func(value string, skipIfInTitle bool) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if skipIfInTitle && strings.Contains(strings.ToLower(parsed.Title), strings.ToLower(value)) {
			return
		}
		for _, existing := range parts {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		parts = append(parts, value)
	}

	add(alertAttributeValue(alert, "Service"), false)
	add(parsed.Labels["kube_cluster_name"], true)
	add(parsed.Labels["kube_namespace"], true)
	add(parsed.Labels["kube_deployment"], true)
	return strings.Join(parts, " · ")
}

func formatAlertLine(alert incidentio.Alert) string {
	parsed := parseAlertTitle(alert.Title)
	title := parsed.Title
	if title == "" {
		title = strings.Join(strings.Fields(alert.Title), " ")
	}
	title = escapeMrkdwn(title)

	var b strings.Builder
	b.WriteString("• ")
	if parsed.Severity != "" {
		b.WriteString("*")
		b.WriteString(parsed.Severity)
		b.WriteString("*  ")
	}
	b.WriteString(title)
	if href, label := alertSourceLink(alert); href != "" {
		b.WriteString(fmt.Sprintf("  <%s|%s>", href, label))
	}

	if meta := alertMetaLine(alert, parsed); meta != "" {
		b.WriteString("\n    _")
		b.WriteString(meta)
		b.WriteString("_")
	}
	return b.String()
}

func alertSourceLink(alert incidentio.Alert) (string, string) {
	if isDatadogURL(alert.SourceURL) {
		return alert.SourceURL, ":datadog: Datadog"
	}
	if alert.ID != "" {
		return fmt.Sprintf(alertPermalinkFmt, alert.ID), slack.IncidentLinkText
	}
	return "", ""
}

func isDatadogURL(raw string) bool {
	u := strings.ToLower(strings.TrimSpace(raw))
	return strings.Contains(u, "datadoghq.com") || strings.Contains(u, "ddog-gov.com")
}

func escapeMrkdwn(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func sortAlertsByCreatedDesc(alerts []incidentio.Alert) {
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].CreatedAt.After(alerts[j].CreatedAt)
	})
}

func collectTeamNames(alerts []incidentio.Alert, attributeID string) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, alert := range alerts {
		name := alert.TeamName(attributeID)
		if name == "-" || name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func activeDaysOnly(days []dayAlerts) []dayAlerts {
	active := make([]dayAlerts, 0, len(days))
	for _, day := range days {
		if len(day.alerts) == 0 {
			continue
		}
		sortAlertsByCreatedDesc(day.alerts)
		active = append(active, day)
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].date.After(active[j].date)
	})
	return active
}

func summaryLine(days []dayAlerts, total int, daysBack int, attributeID string) string {
	teamNames := collectTeamNames(flattenAlerts(days), attributeID)
	line := fmt.Sprintf("_%d alerta(s) firing · últimos %d dias_", total, daysBack)
	if len(teamNames) > 0 {
		line += fmt.Sprintf(" · _%s_", strings.Join(teamNames, ", "))
	}
	return line
}

func formatDaySection(day dayAlerts) string {
	noun := "alerta"
	if len(day.alerts) != 1 {
		noun = "alertas"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%s*  ·  %d %s\n", day.label, len(day.alerts), noun))
	for i, alert := range day.alerts {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(formatAlertLine(alert))
	}
	return b.String()
}

func buildDailyAlertSections(days []dayAlerts) []string {
	activeDays := activeDaysOnly(days)
	sections := make([]string, 0, len(activeDays))
	for _, day := range activeDays {
		text := formatDaySection(day)
		if utf8Len(text) <= slackSectionMax {
			sections = append(sections, text)
			continue
		}
		sections = append(sections, splitLongDaySection(day)...)
	}
	return sections
}

func splitLongDaySection(day dayAlerts) []string {
	sections := make([]string, 0)
	remaining := day.alerts
	part := 1
	for len(remaining) > 0 {
		chunk := dayAlerts{
			date:   day.date,
			label:  fmt.Sprintf("%s (%d)", day.label, part),
			alerts: remaining[:1],
		}
		remaining = remaining[1:]
		for len(remaining) > 0 {
			candidate := chunk
			candidate.alerts = append(append([]incidentio.Alert{}, chunk.alerts...), remaining[0])
			if utf8Len(formatDaySection(candidate)) > slackSectionMax {
				break
			}
			chunk = candidate
			remaining = remaining[1:]
		}
		sections = append(sections, formatDaySection(chunk))
		part++
	}
	return sections
}

func utf8Len(s string) int {
	return len([]rune(s))
}

func flattenAlerts(days []dayAlerts) []incidentio.Alert {
	var all []incidentio.Alert
	for _, day := range days {
		all = append(all, day.alerts...)
	}
	return all
}

func assembleDailyAlertsBlocks(sections []string, footerText string) []slack.MySlackBlock {
	if len(sections) == 0 {
		sections = []string{emptyDailyAlertsSection}
	}
	return slack.AssembleMessage(slack.HeaderDailyAlerts, sections, footerText)
}

func printDailyAlertsPreview(sections []string, footerText string, total int, channel string) {
	if len(sections) == 0 {
		sections = []string{emptyDailyAlertsSection}
	}
	printSlackPreview("Daily Alerts", slack.HeaderDailyAlerts, sections, footerText)
	fmt.Printf("\n(%d alertas · canal %s)\n", total, channel)
}

func buildIncidentAlertsURL(statuses []string, startDay time.Time, attributeID string, catalogIDs []string) string {
	parts := []string{
		fmt.Sprintf("created_at[gte]=%s", startDay.Format("2006-01-02")),
	}

	for _, status := range statuses {
		parts = append(parts, fmt.Sprintf("status[one_of]=%s", status))
	}

	for _, catalogID := range catalogIDs {
		parts = append(parts, fmt.Sprintf("attributes[%s][one_of]=%s", attributeID, catalogID))
	}

	return "https://app.incident.io/on-call/alerts?" + strings.Join(parts, "&")
}

func trimCatalogIDs(ids []string) []string {
	trimmed := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			trimmed = append(trimmed, id)
		}
	}
	return trimmed
}

func dailyAlerts() {
	config := environments.Key()

	if config.IncidentAlertAttributeID == "" || len(config.IncidentAlertCatalogIDs) == 0 {
		logrus.WithField("cmd", "dailyAlerts").Error(
			"INCIDENT_ALERT_ATTRIBUTE_ID e INCIDENT_ALERT_CATALOG_IDS são obrigatórios",
		)
		return
	}

	if config.IncidentAlertDays <= 0 {
		logrus.WithField("cmd", "dailyAlerts").Error("INCIDENT_ALERT_DAYS deve ser maior que zero")
		return
	}

	loc, err := time.LoadLocation(config.IncidentAlertTimezone)
	if err != nil {
		logrus.WithField("cmd", "dailyAlerts").Errorln("Erro ao carregar timezone:", err)
		return
	}

	clientIncidentio, err := incidentio.Client(config.IncidentApiKey)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao criar cliente Incident.io:", err)
		return
	}

	now := time.Now().In(loc)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).
		AddDate(0, 0, -(config.IncidentAlertDays - 1))

	alertFilter := incidentio.AlertListFilter{
		Statuses:    config.IncidentAlertStatuses,
		CreatedGte:  startDay,
		AttributeID: config.IncidentAlertAttributeID,
		CatalogIDs:  trimCatalogIDs(config.IncidentAlertCatalogIDs),
	}

	rawAlerts, err := clientIncidentio.ListAllAlerts(alertFilter)
	if err != nil {
		return
	}

	alerts := incidentio.FilterAlertsByTeam(
		rawAlerts,
		alertFilter.AttributeID,
		alertFilter.CatalogIDs,
	)

	dayMap := make(map[string]*dayAlerts, config.IncidentAlertDays)
	for i := 0; i < config.IncidentAlertDays; i++ {
		day := startDay.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		dayMap[key] = &dayAlerts{
			date:   day,
			label:  abv(day) + " " + day.Format("02/01"),
			alerts: []incidentio.Alert{},
		}
	}

	for _, alert := range alerts {
		key := alert.CreatedAt.In(loc).Format("2006-01-02")
		if entry, ok := dayMap[key]; ok {
			entry.alerts = append(entry.alerts, alert)
		}
	}

	days := make([]dayAlerts, 0, config.IncidentAlertDays)
	for i := 0; i < config.IncidentAlertDays; i++ {
		key := startDay.AddDate(0, 0, i).Format("2006-01-02")
		days = append(days, *dayMap[key])
	}

	alertsURL := buildIncidentAlertsURL(
		config.IncidentAlertStatuses,
		startDay,
		config.IncidentAlertAttributeID,
		alertFilter.CatalogIDs,
	)
	summary := summaryLine(days, len(alerts), config.IncidentAlertDays, config.IncidentAlertAttributeID)
	footerText := slack.ComposeFooter("Ver todos", alertsURL, summary)
	sections := buildDailyAlertSections(days)

	blocks := slack.ConvertToSlackBlocks(assembleDailyAlertsBlocks(sections, footerText))

	if dailyAlertsDryRun {
		printDailyAlertsPreview(sections, footerText, len(alerts), config.SlackChannleIdOperation)
		return
	}

	clientSlack, err := slack.Client(config.SlackApiKey)
	if err != nil {
		logrus.WithField("service", "slack").Error("Erro ao criar cliente Slack:", err)
		return
	}

	if config.SlackChannleIdOperation == "" {
		logrus.WithField("cmd", "dailyAlerts").Error("SLACK_CHANNEL_ID_OPERATION é obrigatório para envio")
		return
	}

	_, _, _, err = clientSlack.Client.SendMessage(config.SlackChannleIdOperation, slack.ChannelMsgOptions(blocks...)...)
	if err != nil {
		logrus.WithField("cmd", "dailyAlerts").Errorf("Erro ao enviar mensagem para o Slack: %s", err)
		return
	}

	logrus.WithField("cmd", "dailyAlerts").Info("Resumo diário de alertas enviado ao Slack")
}

var dailyAlertsCmd = &cobra.Command{
	Use:   "dailyAlerts",
	Short: "Envia resumo diário de alertas do Incident.io para o Slack",
	Long: `Busca alertas no Incident.io com os filtros configurados via env
e envia um resumo agrupado por dia para o canal operacional do Slack.

Use --dry-run para visualizar a mensagem sem enviar.`,
	Example: "opslink dailyAlerts --dry-run",
	Run:     runDailyAlerts,
}

func runDailyAlerts(cmd *cobra.Command, args []string) {
	dailyAlerts()
}

func init() {
	dailyAlertsCmd.Flags().BoolVar(&dailyAlertsDryRun, "dry-run", false, "Exibe a mensagem no terminal sem enviar ao Slack")
	rootCmd.AddCommand(dailyAlertsCmd)
}
