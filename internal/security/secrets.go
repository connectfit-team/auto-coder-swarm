package security

import (
	"context"
	"regexp"
	"strings"
)

type SecretScanner struct{}

func (s *SecretScanner) Name() string { return "SecretScanner" }

func (s *SecretScanner) Scan(ctx context.Context, repoPath string, diff string) ([]Finding, error) {
	var findings []Finding

	// Standard patterns for common secrets
	patterns := map[string]*regexp.Regexp{
		"AWS Access Key": regexp.MustCompile(`(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`),
		"Generic Secret": regexp.MustCompile(`(?i)(secret|password|passwd|api_key|token|auth|key)[\s:=]+['"][A-Za-z0-9/\+=]{16,}['"]`),
		"Private Key":    regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----`),
		"Google API Key": regexp.MustCompile(`AIza[0-9A-Za-z-_]{35}`),
	}

	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		// Only check added lines in diff
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}

		for name, re := range patterns {
			if re.MatchString(line) {
				findings = append(findings, Finding{
					ScannerName: s.Name(),
					Level:       LevelCritical,
					Message:     "Potential secret detected: " + name,
					Line:        i + 1,
				})
			}
		}
	}

	return findings, nil
}
