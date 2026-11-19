package cmd

import (
	"fmt"
	"opslink/internal/environments"
	"opslink/internal/incidentio"
	"opslink/internal/slack"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var nextOncallDryRun bool

func abv(t time.Time) string {
	diaSemana := t.Weekday().String()
	switch diaSemana {
	case "Sunday":
		return "Dom"
	case "Monday":
		return "Seg"
	case "Tuesday":
		return "Ter"
	case "Wednesday":
		return "Qua"
	case "Thursday":
		return "Qui"
	case "Friday":
		return "Sex"
	case "Saturday":
		return "Sáb"
	default:
		return ""
	}
}

type oncallDayEntry struct {
	Date      string
	Primary   string
	Secondary string
}

func ensureOncallDay(dayMap map[string]*oncallDayEntry, start time.Time) *oncallDayEntry {
	dateKey := start.Format("2006-01-02")
	if dayMap[dateKey] == nil {
		dayMap[dateKey] = &oncallDayEntry{
			Date: abv(start) + " " + start.Format("02/01"),
		}
	}
	return dayMap[dateKey]
}

func nextOncall() {
	config := environments.Key()

	clientIncidentio, err := incidentio.Client(config.IncidentApiKey)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao criar cliente Incident.io:", err)
		return
	}

	loc, err := time.LoadLocation("America/Recife")
	if err != nil {
		logrus.WithField("cmd", "nextOncall").Errorln("Erro ao carregar o fuso horário de Recife:", err)
		return
	}

	now := time.Now().UTC()
	startTime := now.Format(time.RFC3339)
	endTime := now.AddDate(0, 0, 7).Format(time.RFC3339)

	dayMap := make(map[string]*oncallDayEntry)

	if config.IncidentSchedulePrimary != "" {
		primaryEntries, err := clientIncidentio.GetScheduleEntries(config.IncidentSchedulePrimary, startTime, endTime)
		if err != nil {
			logrus.WithField("cmd", "nextOncall").Error("Erro ao buscar schedule entries primário:", err)
		} else {
			for _, entry := range primaryEntries.ScheduleEntries.Final {
				start := entry.StartAt.In(loc)
				ensureOncallDay(dayMap, start).Primary = formatOncallMention(entry.User.SlackUserID)
			}
		}
	}

	if config.IncidentScheduleSecondary != "" {
		secondaryEntries, err := clientIncidentio.GetScheduleEntries(config.IncidentScheduleSecondary, startTime, endTime)
		if err != nil {
			logrus.WithField("cmd", "nextOncall").Error("Erro ao buscar schedule entries secundário:", err)
		} else {
			for _, entry := range secondaryEntries.ScheduleEntries.Final {
				start := entry.StartAt.In(loc)
				ensureOncallDay(dayMap, start).Secondary = formatOncallMention(entry.User.SlackUserID)
			}
		}
	}

	sections := buildNextOncallSections(now, dayMap)
	if len(sections) == 0 {
		sections = []string{"_Nenhuma escala encontrada para os próximos 7 dias._"}
	}

	footer := slack.ComposeFooter(
		"Mais detalhes sobre a escala da semana",
		slack.IncidentSchedulesURL,
		"*⚠️ Caso precise fazer override, combine antecipadamente com seu pair.*",
	)

	if nextOncallDryRun {
		printSlackPreview("Next Oncall", slack.HeaderNextOncall, sections, footer)
		fmt.Printf("\n(canal %s)\n", config.SlackChannleIdOperation)
		return
	}

	clientSlack, err := slack.Client(config.SlackApiKey)
	if err != nil {
		logrus.WithField("service", "slack").Error("Erro ao criar cliente Slack:", err)
		return
	}

	if config.SlackChannleIdOperation == "" {
		logrus.WithField("cmd", "nextOncall").Error("SLACK_CHANNEL_ID_OPERATION é obrigatório para envio")
		return
	}

	blocks := slack.ConvertToSlackBlocks(slack.AssembleMessage(slack.HeaderNextOncall, sections, footer))
	_, _, _, err = clientSlack.Client.SendMessage(config.SlackChannleIdOperation, slack.ChannelMsgOptions(blocks...)...)
	if err != nil {
		logrus.WithField("cmd", "nextOncall").Errorf("Erro ao enviar a mensagem para o Slack: %s\n", err)
		return
	}
}

var nextOncallCmd = &cobra.Command{
	Use:   "nextOncall",
	Short: "Retorna usuários da escala da semana",
	Long: `Exibe em uma timeline usuários que está na escala de plantão. 
Essa timeline também ira exibir os overrides que existem, então pode 
ocorrer de aparecer mais de uma linha do mesmo dia.

Use --dry-run para visualizar a mensagem sem enviar.`,
	Example: "opslink nextOncall --dry-run",
	Run:     runNextOncall,
}

func buildNextOncallSections(now time.Time, dayMap map[string]*oncallDayEntry) []string {
	sections := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, i)
		entry, exists := dayMap[day.Format("2006-01-02")]
		if !exists {
			continue
		}
		section := formatOncallDaySection(entry.Date, entry.Primary, entry.Secondary)
		if section == "" {
			continue
		}
		sections = append(sections, section)
	}
	return sections
}

func runNextOncall(cmd *cobra.Command, args []string) {
	nextOncall()
}

func init() {
	nextOncallCmd.Flags().BoolVar(&nextOncallDryRun, "dry-run", false, "Exibe a mensagem no terminal sem enviar ao Slack")
	rootCmd.AddCommand(nextOncallCmd)
}
