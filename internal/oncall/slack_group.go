package oncall

import (
	"strings"
)

func BuildSlackGroupMemberIDs(primarySecondary []string, additionalIDs []string) []string {
	members := make([]string, 0, len(primarySecondary)+len(additionalIDs))
	members = append(members, trimNonEmpty(primarySecondary)...)
	members = append(members, trimNonEmpty(additionalIDs)...)
	return dedupeStrings(members)
}

func PlanSlackGroup(primaryID, secondaryID string, additionalIDs []string) []string {
	return BuildSlackGroupMemberIDs([]string{primaryID, secondaryID}, additionalIDs)
}

func trimNonEmpty(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
