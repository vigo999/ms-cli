package project

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	milestoneTodo       = "todo"
	milestoneInProgress = "in_progress"
	milestoneDone       = "done"
	milestoneBlocked    = "blocked"
	milestoneUnknown    = "unknown"
)

type Roadmap struct {
	Version    int     `yaml:"version" json:"version"`
	TargetDate string  `yaml:"target_date" json:"target_date"`
	Phases     []Phase `yaml:"phases" json:"phases"`
}

type Phase struct {
	ID         string      `yaml:"id" json:"id"`
	Name       string      `yaml:"name" json:"name"`
	Start      string      `yaml:"start" json:"start"`
	End        string      `yaml:"end" json:"end"`
	Milestones []Milestone `yaml:"milestones" json:"milestones"`
}

type Milestone struct {
	ID     string `yaml:"id" json:"id"`
	Title  string `yaml:"title" json:"title"`
	Status string `yaml:"status" json:"status"`
}

type RoadmapStatus struct {
	TargetDate  string        `json:"target_date"`
	GeneratedAt time.Time     `json:"generated_at"`
	Overall     Progress      `json:"overall"`
	Phases      []PhaseStatus `json:"phases"`
}

type PhaseStatus struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Window   string   `json:"window"`
	Progress Progress `json:"progress"`
	Items    int      `json:"items"`
	Done     int      `json:"done"`
	InProg   int      `json:"in_progress"`
	Todo     int      `json:"todo"`
	Blocked  int      `json:"blocked"`
}

type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
	Pct   int `json:"pct"`
}

// LoadRoadmapFromFile loads and validates roadmap YAML from a file path.
func LoadRoadmapFromFile(path string) (*Roadmap, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("roadmap path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read roadmap file %q: %w", path, err)
	}

	var rm Roadmap
	if err := yaml.Unmarshal(data, &rm); err != nil {
		return nil, fmt.Errorf("parse roadmap yaml %q: %w", path, err)
	}

	if err := validateRoadmap(&rm); err != nil {
		return nil, err
	}

	return &rm, nil
}

// ComputeRoadmapStatus computes aggregate progress for phases and overall roadmap.
func ComputeRoadmapStatus(rm *Roadmap, now time.Time) (*RoadmapStatus, error) {
	if err := validateRoadmap(rm); err != nil {
		return nil, err
	}

	status := &RoadmapStatus{
		TargetDate:  rm.TargetDate,
		GeneratedAt: now.UTC(),
		Phases:      make([]PhaseStatus, 0, len(rm.Phases)),
	}

	overallDone := 0
	overallTotal := 0

	for _, phase := range rm.Phases {
		phaseStatus := computePhaseStatus(phase)
		status.Phases = append(status.Phases, phaseStatus)
		overallDone += phaseStatus.Done
		overallTotal += phaseStatus.Items
	}

	status.Overall = computeProgress(overallDone, overallTotal)
	return status, nil
}

// IsComplete reports whether the roadmap is fully complete.
func (r *RoadmapStatus) IsComplete() bool {
	if r == nil {
		return false
	}
	return r.Overall.Pct == 100
}

func validateRoadmap(rm *Roadmap) error {
	if rm == nil {
		return errors.New("roadmap is nil")
	}
	if rm.Version <= 0 {
		return fmt.Errorf("invalid roadmap version: %d", rm.Version)
	}
	if strings.TrimSpace(rm.TargetDate) == "" {
		return errors.New("roadmap target_date is required")
	}

	for pi, phase := range rm.Phases {
		if strings.TrimSpace(phase.ID) == "" {
			return fmt.Errorf("phase[%d] id is required", pi)
		}
		if strings.TrimSpace(phase.Name) == "" {
			return fmt.Errorf("phase[%d] name is required", pi)
		}

		for mi, ms := range phase.Milestones {
			if strings.TrimSpace(ms.ID) == "" {
				return fmt.Errorf("phase[%d] milestone[%d] id is required", pi, mi)
			}
			if strings.TrimSpace(ms.Title) == "" {
				return fmt.Errorf("phase[%d] milestone[%d] title is required", pi, mi)
			}
		}
	}

	return nil
}

func computePhaseStatus(phase Phase) PhaseStatus {
	ps := PhaseStatus{
		ID:     phase.ID,
		Name:   phase.Name,
		Window: buildPhaseWindow(phase.Start, phase.End),
	}

	for _, ms := range phase.Milestones {
		ps.Items++
		switch normalizeStatus(ms.Status) {
		case milestoneDone:
			ps.Done++
		case milestoneInProgress:
			ps.InProg++
		case milestoneTodo:
			ps.Todo++
		case milestoneBlocked:
			ps.Blocked++
		case milestoneUnknown:
			// Unknown values count toward total items but not known buckets.
		}
	}

	ps.Progress = computeProgress(ps.Done, ps.Items)
	return ps
}

func computeProgress(done, total int) Progress {
	p := Progress{Done: done, Total: total}
	if total <= 0 {
		return p
	}
	p.Pct = (done * 100) / total
	return p
}

func normalizeStatus(status string) string {
	s := strings.TrimSpace(strings.ToLower(status))
	switch s {
	case milestoneTodo, milestoneInProgress, milestoneDone, milestoneBlocked:
		return s
	default:
		return milestoneUnknown
	}
}

func buildPhaseWindow(start, end string) string {
	s := strings.TrimSpace(start)
	e := strings.TrimSpace(end)

	switch {
	case s == "" && e == "":
		return ""
	case s == "":
		return e
	case e == "":
		return s
	default:
		return s + " -> " + e
	}
}

/*
Example usage:

	rm, err := project.LoadRoadmapFromFile("roadmap.yaml")
	if err != nil {
		// handle error
	}

	st, err := project.ComputeRoadmapStatus(rm, time.Now())
	if err != nil {
		// handle error
	}

	_ = st // JSON-marshallable status object
*/
