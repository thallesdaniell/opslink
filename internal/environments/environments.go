package environments

import (
	"github.com/caarlos0/env"
	"github.com/sirupsen/logrus"
)

type Environments struct {
	ServiceAccount            string   `env:"GOOGLE_APPLICATION_CREDENTIALS"`
	SubjectEmail              string   `env:"GOOGLE_SUBJECT_EMAIL"`
	GroupKey                  string   `env:"GOOGLE_GROUP_KEY"`
	SlackGroupIdUpdateOncall  string   `env:"SLACK_GROUP_ID_UPDATE_ONCALL"`
	SlackChannleIdOperation   string   `env:"SLACK_CHANNEL_ID_OPERATION"`
	SlackGroupIdSupport       string   `env:"SLACK_GROUP_ID_SUPPORT"`
	SlackApiKey               string   `env:"SLACK_BOT_TOKEN"`
	SlackAdditionalUserIDs    []string `env:"SLACK_ADDITIONAL_USERS_GROUP" envSeparator:","`
	IncidentApiKey            string   `env:"INCIDENT_API_KEY"`
	IncidentSchedulePrimary   string   `env:"INCIDENT_SCHEDULE_PRIMARY"`
	IncidentScheduleSecondary string   `env:"INCIDENT_SCHEDULE_SECONDARY"`
	IncidentAlertAttributeID  string   `env:"INCIDENT_ALERT_ATTRIBUTE_ID"`
	IncidentAlertCatalogIDs   []string `env:"INCIDENT_ALERT_CATALOG_IDS" envSeparator:","`
	IncidentAlertStatuses     []string `env:"INCIDENT_ALERT_STATUS" envDefault:"firing" envSeparator:","`
	IncidentAlertDays         int      `env:"INCIDENT_ALERT_DAYS" envDefault:"21"`
	IncidentAlertTimezone     string   `env:"INCIDENT_ALERT_TIMEZONE" envDefault:"UTC"`
	GeminiAPIKey              string   `env:"GEMINI_API_KEY"`
	GeminiModel               string   `env:"GEMINI_MODEL" envDefault:"gemini-2.5-flash"`
	GcpEmailSheetID           string   `env:"GCP_EMAIL_SHEET_ID"`
	GcpEmailSheetRange        string   `env:"GCP_EMAIL_SHEET_RANGE" envDefault:"Emails!A2:F"`
}

func Key() Environments {
	envs := Environments{}
	err := env.Parse(&envs)
	if err != nil {
		logrus.Fatalf("Erro ao fazer o parse: %s", err.Error())
	}
	return envs
}
