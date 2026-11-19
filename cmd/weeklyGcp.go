package cmd

import (
	"fmt"
	"net/url"
	"opslink/internal/environments"
	"opslink/internal/gemini"
	"opslink/internal/gmail"
	"opslink/internal/gsheets"
	"opslink/internal/slack"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	emptyWeeklyGcpSection = "_Nenhum comunicado operacional da GCP nesta semana._"
)

var weeklyGcpDryRun bool

func weeklyGcp() {
	config := environments.Key()

	if config.GeminiAPIKey == "" {
		logrus.WithField("cmd", "weeklyGcp").Error("GEMINI_API_KEY é obrigatório")
		return
	}
	if config.ServiceAccount == "" {
		logrus.WithField("cmd", "weeklyGcp").Error("GOOGLE_APPLICATION_CREDENTIALS é obrigatório")
		return
	}
	if strings.TrimSpace(config.GcpEmailSheetID) == "" {
		logrus.WithField("cmd", "weeklyGcp").Error("GCP_EMAIL_SHEET_ID é obrigatório")
		return
	}

	loc, err := time.LoadLocation(config.IncidentAlertTimezone)
	if err != nil {
		logrus.WithField("cmd", "weeklyGcp").Errorln("Erro ao carregar timezone:", err)
		return
	}

	now := time.Now().In(loc)
	periodStart := now.AddDate(0, 0, -7)

	sheetClient, err := gsheets.Service(config.ServiceAccount)
	if err != nil {
		return
	}

	messages, err := sheetClient.ReadMessages(config.GcpEmailSheetID, config.GcpEmailSheetRange)
	if err != nil {
		return
	}

	report := &gemini.Report{}
	if len(messages) > 0 {
		summarized, err := gemini.New(config.GeminiAPIKey, config.GeminiModel).SummarizeWeekly(gmail.FormatDigest(messages))
		if err != nil {
			logrus.WithField("cmd", "weeklyGcp").Error("IA não retornou resumo:", err)
			return
		}
		report = summarized
	}

	sections := buildWeeklyGcpSections(report)
	footer := weeklyGcpFooter(len(messages), periodStart, now, report.Resolvidos)

	if weeklyGcpDryRun {
		printSlackPreview("Weekly GCP", slack.HeaderWeeklyGcp, sections, footer)
		fmt.Printf("\n(%d emails · canal %s)\n", len(messages), config.SlackChannleIdOperation)
		return
	}

	clientSlack, err := slack.Client(config.SlackApiKey)
	if err != nil {
		logrus.WithField("service", "slack").Error("Erro ao criar cliente Slack:", err)
		return
	}
	if config.SlackChannleIdOperation == "" {
		logrus.WithField("cmd", "weeklyGcp").Error("SLACK_CHANNEL_ID_OPERATION é obrigatório para envio")
		return
	}

	blocks := slack.ConvertToSlackBlocks(slack.AssembleMessage(slack.HeaderWeeklyGcp, sections, footer))
	_, _, _, err = clientSlack.Client.SendMessage(config.SlackChannleIdOperation, slack.ChannelMsgOptions(blocks...)...)
	if err != nil {
		logrus.WithField("cmd", "weeklyGcp").Errorf("Erro ao enviar a mensagem para o Slack: %s", err)
		return
	}
	logrus.WithField("cmd", "weeklyGcp").Info("Resumo semanal GCP enviado ao Slack")
}

func buildWeeklyGcpSections(report *gemini.Report) []string {
	if report == nil {
		return []string{emptyWeeklyGcpSection}
	}

	sections := make([]string, 0, 3)
	if text := formatGcpWeeklySection(":red_circle:", "Crítico", "evento", "eventos", report.Critico); text != "" {
		sections = append(sections, text)
	}
	if text := formatGcpWeeklySection(":large_orange_circle:", "Relevante", "evento", "eventos", report.Relevante); text != "" {
		sections = append(sections, text)
	}
	if text := formatGcpWeeklySection(":hammer_and_wrench:", "Ações", "ação", "ações", report.Acoes); text != "" {
		sections = append(sections, text)
	}
	if len(sections) == 0 {
		if report.Resolvidos > 0 {
			return []string{":large_green_circle:  _Sem pendências operacionais nesta semana._"}
		}
		return []string{emptyWeeklyGcpSection}
	}
	return sections
}

func formatGcpWeeklySection(icon, title, singular, plural string, items []gemini.Item) string {
	if len(items) == 0 {
		return ""
	}
	noun := singular
	if len(items) != 1 {
		noun = plural
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*%s %s*  ·  %d %s\n", icon, title, len(items), noun)
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(formatGcpWeeklyLine(item))
	}
	return b.String()
}

func formatGcpWeeklyLine(item gemini.Item) string {
	var b strings.Builder
	b.WriteString("• ")
	b.WriteString(escapeMrkdwn(item.Title))
	if href := safeHTTPURL(item.Link); href != "" {
		fmt.Fprintf(&b, "  <%s|%s>", href, gcpLinkLabel(href))
	}
	if meta := gcpWeeklyMeta(item); meta != "" {
		b.WriteString("\n    _")
		b.WriteString(escapeMrkdwn(meta))
		b.WriteString("_")
	}
	return b.String()
}

func gcpWeeklyMeta(item gemini.Item) string {
	parts := make([]string, 0, 2)
	if item.Product != "" {
		parts = append(parts, item.Product)
	}
	if item.Detail != "" && !strings.EqualFold(item.Detail, item.Title) {
		parts = append(parts, item.Detail)
	}
	return strings.Join(parts, " · ")
}

func gcpLinkLabel(raw string) string {
	u := strings.ToLower(raw)
	switch {
	case strings.Contains(u, "status.cloud.google.com"):
		return "status"
	case strings.Contains(u, "console.cloud.google.com"):
		return "console"
	case strings.Contains(u, "mail.google.com"):
		return "email"
	default:
		return "link"
	}
}

func safeHTTPURL(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if parsed.Host == "" {
		return ""
	}
	return raw
}

func weeklyGcpFooter(emailCount int, start, end time.Time, resolvidos int) string {
	period := fmt.Sprintf("%s → %s", start.Format("02/01"), end.Format("02/01"))
	meta := fmt.Sprintf("*⚠️ %d email(s) · %s.*", emailCount, period)
	if resolvidos > 0 {
		meta = fmt.Sprintf("*⚠️ %d email(s) · %s · %d já resolvido(s).*", emailCount, period, resolvidos)
	}
	return slack.ComposeLinkFooter(
		":gcp:",
		"Comunicações oficiais",
		slack.GCPStatusURL,
		"GCP Status",
		meta,
	)
}

var weeklyGcpCmd = &cobra.Command{
	Use:   "weeklyGcp",
	Short: "Envia resumo semanal de comunicações GCP / Kubernetes",
	Long: `Lê o dump semanal de emails (planilha preenchida pelo Apps Script),
resume com Gemini e envia um briefing curto para o canal operacional.

Use --dry-run para visualizar a mensagem sem enviar.`,
	Example: "opslink weeklyGcp --dry-run",
	Run:     runWeeklyGcp,
}

func runWeeklyGcp(cmd *cobra.Command, args []string) {
	weeklyGcp()
}

func init() {
	weeklyGcpCmd.Flags().BoolVar(&weeklyGcpDryRun, "dry-run", false, "Exibe a mensagem no terminal sem enviar ao Slack")
	rootCmd.AddCommand(weeklyGcpCmd)
}
