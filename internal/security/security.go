package security

import (
	"context"
	"fmt"
)

// Level represents the severity of a security finding.
type Level string

const (
	LevelCritical Level = "CRITICAL"
	LevelHigh     Level = "HIGH"
	LevelMedium   Level = "MEDIUM"
	LevelLow      Level = "LOW"
)

// Finding represents a single security issue detected in the code or diff.
type Finding struct {
	ScannerName string `json:"scanner_name"`
	Level       Level  `json:"level"`
	Message     string `json:"message"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// Scanner is the interface for modular security tools.
type Scanner interface {
	Name() string
	// Scan analyzes a repository path or a specific diff.
	Scan(ctx context.Context, repoPath string, diff string) ([]Finding, error)
}

// Guardrail manages multiple scanners and aggregates findings.
type Guardrail struct {
	scanners []Scanner
}

func NewGuardrail(scanners ...Scanner) *Guardrail {
	return &Guardrail{scanners: scanners}
}

func (g *Guardrail) ExecuteAll(ctx context.Context, repoPath string, diff string) ([]Finding, error) {
	var allFindings []Finding
	for _, s := range g.scanners {
		findings, err := s.Scan(ctx, repoPath, diff)
		if err != nil {
			// We log but continue other scans for maximum visibility
			fmt.Printf("[Security] %s scan failed: %v\n", s.Name(), err)
			continue
		}
		allFindings = append(allFindings, findings...)
	}
	return allFindings, nil
}
