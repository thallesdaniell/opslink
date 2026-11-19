package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "opslink",
	Short: "Sincroniza plantão e alertas entre Incident.io, Google Workspace e Slack",
	Long: `Opslink mantém o grupo Google e o @ do Slack alinhados ao Incident.io,
envia resumo de alertas firing, a rotação semanal e o briefing GCP/K8s.

Comandos: updateOncall, dailyAlerts, nextOncall, weeklyGcp`,
	Example: "opslink updateOncall --dry-run",
	Version: "1.0.0",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
