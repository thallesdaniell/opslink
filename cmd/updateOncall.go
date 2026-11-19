package cmd

import (
	"fmt"
	"opslink/internal/environments"
	"opslink/internal/gcp"
	"opslink/internal/incidentio"
	"opslink/internal/oncall"
	"opslink/internal/slack"
	"os"
	"slices"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
	slackgo "github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

func init() {

	if os.Getenv("APP_ENV") == "production" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&nested.Formatter{
			HideKeys:        true,
			TimestampFormat: time.RFC3339Nano,
		})
	}
}

var (
	updateOncallDryRun                 bool
	updateOncallNotify                 bool
	updateOncallPreviousPrimarySlack   string
	updateOncallPreviousSecondarySlack string
)

type OncallUser struct {
	Email       string
	Name        string
	SlackUserID string
	Slack       *slackgo.User
}

func getOncallUsers(config environments.Environments) (map[string]OncallUser, error) {

	clientIncidentio, err := incidentio.Client(config.IncidentApiKey)
	if err != nil {
		return nil, err
	}

	oncall := map[string]OncallUser{}

	scheduleTypes := []struct {
		scheduleID string
		key        string
	}{
		{config.IncidentSchedulePrimary, "primary"},
		{config.IncidentScheduleSecondary, "secondary"},
	}

	for _, scheduleType := range scheduleTypes {
		if scheduleType.scheduleID == "" {
			continue
		}

		schedule, err := clientIncidentio.GetScheduleWithCurrentShifts(scheduleType.scheduleID)
		if err != nil {
			logrus.WithField("service", "incidentio").Errorf("Erro ao buscar %s: %v", scheduleType.key, err)
			return nil, fmt.Errorf("incident.io indisponível para %s: %w", scheduleType.key, err)
		}

		user, ok, userErr := oncallUserFromSchedule(scheduleType.key, schedule)
		if userErr != nil {
			return nil, userErr
		}
		if ok {
			oncall[scheduleType.key] = user
		}
	}

	return oncall, nil
}

// oncallUserFromSchedule maps a schedule's current shift. Primary must have a
// live shift. Secondary is optional (nights 18:00–09:00 and weekends).
func oncallUserFromSchedule(key string, schedule *incidentio.Schedule) (OncallUser, bool, error) {
	if schedule == nil || len(schedule.CurrentShifts) == 0 {
		if key == "primary" {
			name := "primário"
			if schedule != nil && schedule.Name != "" {
				name = schedule.Name
			}
			return OncallUser{}, false, fmt.Errorf("schedule %s sem plantão atual", name)
		}
		name := "secundário"
		if schedule != nil && schedule.Name != "" {
			name = schedule.Name
		}
		logrus.WithField("service", "incidentio").Infof(
			"Schedule %s sem plantão atual — secundário omitido (opcional fora do horário / fim de semana)",
			name,
		)
		return OncallUser{}, false, nil
	}

	shift := schedule.CurrentShifts[0]
	return OncallUser{
		Email:       shift.User.Email,
		Name:        shift.User.Name,
		SlackUserID: shift.User.SlackUserID,
	}, true, nil
}

func oncallWouldChange(currentOncall map[string]OncallUser, currentUsersGcp []string) bool {
	for _, user := range currentOncall {
		if user.Email != "" && !slices.Contains(currentUsersGcp, user.Email) {
			return true
		}
	}
	for _, userGcp := range currentUsersGcp {
		found := false
		for _, user := range currentOncall {
			if user.Email == userGcp {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}

func slackGroupMemberIDsFromOncall(
	config environments.Environments,
	currentOncall map[string]OncallUser,
) []string {
	primaryID := ""
	secondaryID := ""
	if u, ok := currentOncall["primary"]; ok {
		primaryID = u.SlackUserID
	}
	if u, ok := currentOncall["secondary"]; ok {
		secondaryID = u.SlackUserID
	}

	return oncall.PlanSlackGroup(primaryID, secondaryID, config.SlackAdditionalUserIDs)
}

func buildOncallSlackBlocks(currentOncall map[string]OncallUser, primaryOldID, secondaryOldID string) []slackgo.Block {
	message := formatOncallRoleLines(
		formatOncallMention(currentOncall["primary"].SlackUserID),
		formatOncallMention(currentOncall["secondary"].SlackUserID),
	)

	return slack.ConvertToSlackBlocks(slack.AssembleMessage(
		slack.HeaderOncallUpdate,
		[]string{message},
		previousOncallFooter(primaryOldID, secondaryOldID),
	))
}

func previousOncallFooter(primaryOldID, secondaryOldID string) string {
	var thanks string
	switch {
	case primaryOldID != "" && secondaryOldID != "":
		thanks = fmt.Sprintf(
			"*⚠️ Obrigado pelo plantão anterior <@%s> e <@%s>! Se houve algo importante que precise ser registrado, informe nesta thread.*",
			primaryOldID, secondaryOldID,
		)
	case primaryOldID != "":
		thanks = fmt.Sprintf(
			"*⚠️ Obrigado pelo plantão anterior <@%s>! Se houve algo importante que precise ser registrado, informe nesta thread.*",
			primaryOldID,
		)
	}
	return slack.ComposeFooter("Mais detalhes sobre a escala", slack.IncidentSchedulesURL, thanks)
}

func resolvePreviousOncallSlackIDs(
	clientSlack *slack.SlackClient,
	currentUsersGcp []string,
	primaryOverride, secondaryOverride string,
) (string, string, error) {
	if primaryOverride != "" || secondaryOverride != "" {
		return primaryOverride, secondaryOverride, nil
	}

	if len(currentUsersGcp) == 0 {
		return "", "", nil
	}

	primaryOld, err := clientSlack.GetUserByEmail(currentUsersGcp[0])
	if err != nil {
		return "", "", err
	}

	if len(currentUsersGcp) < 2 {
		return primaryOld.ID, "", nil
	}

	secondaryOld, err := clientSlack.GetUserByEmail(currentUsersGcp[1])
	if err != nil {
		return "", "", err
	}

	return primaryOld.ID, secondaryOld.ID, nil
}

func printOncallPreview(blocks []slackgo.Block, channel string, wouldChange bool) {
	fmt.Println("=== Preview Slack (Oncall Updater) ===")
	fmt.Println()
	fmt.Println(slack.HeaderOncallUpdate)
	fmt.Println()
	for _, block := range blocks {
		switch b := block.(type) {
		case *slackgo.SectionBlock:
			if b.Text != nil {
				fmt.Println(b.Text.Text)
			}
		case *slackgo.ContextBlock:
			for _, el := range b.ContextElements.Elements {
				if t, ok := el.(*slackgo.TextBlockObject); ok {
					fmt.Println(t.Text)
				}
			}
		}
	}
	fmt.Println()
	if wouldChange {
		fmt.Println("(em produção: enviaria ao Slack porque o plantão mudou no GCP)")
	} else {
		fmt.Println("(em produção: NÃO enviaria — plantão já está sincronizado)")
	}
	fmt.Printf("(canal %s)\n", channel)
}

func updateOncall() {

	config := environments.Key()
	readOnly := updateOncallDryRun || updateOncallNotify

	currentOncallIncidentio, err := getOncallUsers(config)
	if err != nil {
		logrus.WithField("cmd", "updateOncall").Errorf(
			"Abortando sincronização: não foi possível obter plantão do Incident.io — GCP e Slack permanecem inalterados: %v",
			err,
		)
		return
	}

	clientSlack, err := slack.Client(config.SlackApiKey)
	if err != nil {
		return
	}

	var currentUsersGcp []string
	wouldChange := true

	if readOnly {
		clientGcp, gcpErr := gcp.Service(config.ServiceAccount, config.SubjectEmail)
		if gcpErr == nil {
			currentUsersGcp, err = clientGcp.GetMembers(config.GroupKey)
			if err == nil {
				wouldChange = oncallWouldChange(currentOncallIncidentio, currentUsersGcp)
			} else {
				logrus.WithField("cmd", "updateOncall").Warn("GCP indisponível no modo teste, seguindo sem comparar plantão anterior")
			}
		} else {
			logrus.WithField("cmd", "updateOncall").Warn("GCP indisponível no modo teste, seguindo sem comparar plantão anterior")
		}
	} else {
		clientGcp, err := gcp.Service(config.ServiceAccount, config.SubjectEmail)
		if err != nil {
			return
		}
		currentUsersGcp, err = clientGcp.GetMembers(config.GroupKey)
		if err != nil {
			return
		}
		wouldChange = oncallWouldChange(currentOncallIncidentio, currentUsersGcp)
		change := wouldChange

		for _, user := range currentOncallIncidentio {
			if user.Email == "" {
				continue
			}
			if !slices.Contains(currentUsersGcp, user.Email) {
				clientGcp.InsertMember(config.GroupKey, user.Email)
			}
		}

		for _, userGcp := range currentUsersGcp {
			found := false
			for _, user := range currentOncallIncidentio {
				if user.Email == userGcp {
					found = true
					break
				}
			}
			if !found {
				clientGcp.DeleteMember(config.GroupKey, userGcp)
			}
		}

		memberIDs := slackGroupMemberIDsFromOncall(config, currentOncallIncidentio)
		clientSlack.UpdateUserGroupMembers(config.SlackGroupIdUpdateOncall, memberIDs)

		if !change {
			logrus.WithField("cmd", "updateOncall").Info("Plantão sincronizado, nenhuma mensagem enviada")
			return
		}
	}

	shouldNotify := wouldChange || updateOncallNotify
	if updateOncallDryRun {
		shouldNotify = true
	}

	if !shouldNotify {
		logrus.WithField("cmd", "updateOncall").Info("Plantão sincronizado, nenhuma mensagem enviada")
		return
	}

	primaryOldID, secondaryOldID, err := resolvePreviousOncallSlackIDs(
		clientSlack,
		currentUsersGcp,
		updateOncallPreviousPrimarySlack,
		updateOncallPreviousSecondarySlack,
	)
	if err != nil {
		if readOnly {
			logrus.WithField("cmd", "updateOncall").Warnf("Plantão anterior indisponível: %v (mensagem sem agradecimento)", err)
		} else {
			logrus.WithField("cmd", "updateOncall").Error(err)
			return
		}
	}

	blocks := buildOncallSlackBlocks(currentOncallIncidentio, primaryOldID, secondaryOldID)

	if updateOncallDryRun {
		printOncallPreview(blocks, config.SlackChannleIdOperation, wouldChange)
		return
	}

	if config.SlackChannleIdOperation == "" {
		logrus.WithField("cmd", "updateOncall").Error("SLACK_CHANNEL_ID_OPERATION é obrigatório para envio")
		return
	}

	_, _, _, err = clientSlack.Client.SendMessage(config.SlackChannleIdOperation, slack.ChannelMsgOptions(blocks...)...)
	if err != nil {
		logrus.WithField("cmd", "updateOncall").Errorf("Erro ao enviar a mensagem para o Slack: %s\n", err)
		return
	}

	logrus.WithField("cmd", "updateOncall").Info("Mensagem de plantão enviada ao Slack")
}

var updateOncallCmd = &cobra.Command{
	Use:   "updateOncall",
	Short: "Atualiza grupos do slack e google cloud",
	Long: `Executa o comando updateOncall checando os membros atuais do Incident.io
e compara com os usuários do slack e google cloud, caso estejam diferentes serão atualizados na gcp e no slack.

Use --dry-run para ver a mensagem sem alterar GCP/Slack.
Use --notify para enviar a mensagem ao Slack sem alterar o GCP (útil para testar o canal).
Use --previous-primary-slack e --previous-secondary-slack para simular quem saiu do plantão.

Secundário é opcional: sem plantão atual (noite 18h–9h ou fim de semana) o sync segue só com o primário.`,
	Example: `opslink updateOncall --dry-run
opslink updateOncall --notify`,
	Run: runUpdateOncall,
}

func runUpdateOncall(cmd *cobra.Command, args []string) {
	updateOncall()
}

func init() {
	updateOncallCmd.Flags().BoolVar(&updateOncallDryRun, "dry-run", false, "Exibe a mensagem no terminal sem alterar GCP/Slack")
	updateOncallCmd.Flags().BoolVar(&updateOncallNotify, "notify", false, "Envia a mensagem ao Slack sem alterar o GCP (teste)")
	updateOncallCmd.Flags().StringVar(&updateOncallPreviousPrimarySlack, "previous-primary-slack", "", "Slack user ID do primário anterior (teste)")
	updateOncallCmd.Flags().StringVar(&updateOncallPreviousSecondarySlack, "previous-secondary-slack", "", "Slack user ID do secundário anterior (teste)")
	rootCmd.AddCommand(updateOncallCmd)
}
