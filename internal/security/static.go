package security

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

type StaticAnalysisScanner struct{}

func (s *StaticAnalysisScanner) Name() string { return "StaticAnalysisScanner" }

func (s *StaticAnalysisScanner) Scan(ctx context.Context, repoPath string, diff string) ([]Finding, error) {
	var findings []Finding

	// Check if this is a Go project
	if _, err := exec.LookPath("gosec"); err == nil {
		// Example: gosec -fmt json ./...
		// For now, let's just check if it's applicable
		isGo := false
		matches, _ := filepath.Glob(filepath.Join(repoPath, "go.mod"))
		if len(matches) > 0 { isGo = true }

		if isGo {
			// Implementation logic for gosec could go here
			// findings = append(findings, Finding{...})
		}
	}

	// Example logic: Block obvious mock/shell command injection in diffs
	if strings.Contains(diff, "chmod 777") || strings.Contains(diff, "curl | bash") {
		findings = append(findings, Finding{
			ScannerName: s.Name(),
			Level:       LevelHigh,
			Message:     "Insecure command execution detected (chmod 777 or curl|bash)",
		})
	}

	return findings, nil
}
