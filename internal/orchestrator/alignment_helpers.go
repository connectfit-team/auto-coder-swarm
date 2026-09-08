package orchestrator

import "strings"

func equalFoldTrim(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func joinRepos(rs []string) string { return strings.Join(rs, "·") }
