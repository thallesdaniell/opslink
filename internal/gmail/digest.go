package gmail

import (
	"fmt"
	"regexp"
	"strings"
)

var reURL = regexp.MustCompile(`https?://[^\s<>"'\)\]]+`)

type Message struct {
	Subject string
	From    string
	Date    string
	Body    string
	Links   []string
}

func ParseRows(rows [][]interface{}) []Message {
	out := make([]Message, 0, len(rows))
	for _, row := range rows {
		msg := Message{
			Subject: cell(row, 1),
			From:    cell(row, 2),
			Date:    cell(row, 3),
			Body:    cell(row, 4),
			Links:   splitLinks(cell(row, 5)),
		}
		if msg.Subject == "" && msg.Body == "" {
			continue
		}
		if len(msg.Links) == 0 {
			msg.Links = extractURLs(msg.Body)
		}
		out = append(out, msg)
	}
	return out
}

func FormatDigest(messages []Message) string {
	var b strings.Builder
	for i, msg := range messages {
		if i > 0 {
			b.WriteString("\n-------------------------\n")
		}
		fmt.Fprintf(&b, "ASSUNTO: %s\nDE: %s\nDATA: %s\n", msg.Subject, msg.From, msg.Date)
		if len(msg.Links) > 0 {
			fmt.Fprintf(&b, "LINKS: %s\n", strings.Join(msg.Links, " "))
		}
		fmt.Fprintf(&b, "CORPO:\n%s\n", msg.Body)
	}
	return b.String()
}

func extractURLs(text string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, raw := range reURL.FindAllString(text, -1) {
		url := strings.TrimRight(raw, ".,;:)")
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}
	return out
}

func splitLinks(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ' ' || r == '\n' || r == ','
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimRight(p, ".,;:)")
		if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			out = append(out, p)
		}
	}
	return out
}

func cell(row []interface{}, i int) string {
	if i >= len(row) || row[i] == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(row[i]))
}
