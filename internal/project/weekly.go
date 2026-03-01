package project

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type WeeklyUpdate struct {
	Week       string        `yaml:"week" json:"week"`
	Date       string        `yaml:"date" json:"date"`
	Status     string        `yaml:"status" json:"status,omitempty"`
	Highlights []string      `yaml:"highlights" json:"highlights,omitempty"`
	Metrics    WeeklyMetrics `yaml:"metrics" json:"metrics"`
	Blockers   []string      `yaml:"blockers" json:"blockers,omitempty"`
	NextWeek   []string      `yaml:"next_week" json:"next_week,omitempty"`
	Body       string        `json:"body,omitempty"`
}

type WeeklyMetrics struct {
	MilestonesDone  int `yaml:"milestones_done" json:"milestones_done"`
	MilestonesTotal int `yaml:"milestones_total" json:"milestones_total"`
	ProgressPct     int `yaml:"progress_pct" json:"progress_pct"`
}

// LoadWeeklyUpdateFromFile loads a markdown weekly update with YAML front matter.
func LoadWeeklyUpdateFromFile(path string) (*WeeklyUpdate, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("weekly update path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read weekly update file %q: %w", path, err)
	}

	frontMatter, body, err := splitFrontMatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse weekly update front matter %q: %w", path, err)
	}

	var wu WeeklyUpdate
	if err := yaml.Unmarshal([]byte(frontMatter), &wu); err != nil {
		return nil, fmt.Errorf("unmarshal weekly update front matter %q: %w", path, err)
	}

	wu.Body = strings.TrimSpace(body)
	if err := validateWeeklyUpdate(&wu); err != nil {
		return nil, err
	}

	return &wu, nil
}

func validateWeeklyUpdate(wu *WeeklyUpdate) error {
	if wu == nil {
		return errors.New("weekly update is nil")
	}
	if strings.TrimSpace(wu.Week) == "" {
		return errors.New("weekly update week is required")
	}
	if strings.TrimSpace(wu.Date) == "" {
		return errors.New("weekly update date is required")
	}
	if wu.Metrics.MilestonesDone < 0 || wu.Metrics.MilestonesTotal < 0 || wu.Metrics.ProgressPct < 0 {
		return errors.New("weekly update metrics must be non-negative")
	}
	return nil
}

func splitFrontMatter(content string) (string, string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", errors.New("missing starting front matter delimiter '---'")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", errors.New("missing ending front matter delimiter '---'")
	}

	frontMatter := strings.Join(lines[1:end], "\n")
	body := ""
	if end+1 < len(lines) {
		body = strings.Join(lines[end+1:], "\n")
	}
	return frontMatter, body, nil
}
