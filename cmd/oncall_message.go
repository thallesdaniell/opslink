package cmd

import (
	"fmt"
	"strings"
)

const (
	oncallRolePrimary   = "Primário"
	oncallRoleSecondary = "Secundário"
)

func formatOncallMention(slackUserID string) string {
	if slackUserID == "" {
		return ""
	}
	return "<@" + slackUserID + ">"
}

func formatOncallRoleLines(primaryMention, secondaryMention string) string {
	var b strings.Builder
	writeRole := func(role, mention string) {
		if mention == "" {
			return
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(mention)
		b.WriteString("\n")
	}
	writeRole(oncallRolePrimary, primaryMention)
	writeRole(oncallRoleSecondary, secondaryMention)
	return b.String()
}

func formatOncallDaySection(dateLabel, primaryMention, secondaryMention string) string {
	roles := formatOncallRoleLines(primaryMention, secondaryMention)
	if roles == "" {
		return ""
	}
	return "*" + dateLabel + "*\n" + strings.TrimRight(roles, "\n")
}

func printSlackPreview(commandName, header string, sections []string, footer string) {
	fmt.Printf("=== Preview Slack (%s) ===\n\n", commandName)
	fmt.Println(header)
	fmt.Println()
	for i, section := range sections {
		if i > 0 {
			fmt.Println("──")
		}
		fmt.Println(section)
		fmt.Println()
	}
	fmt.Println(footer)
}
