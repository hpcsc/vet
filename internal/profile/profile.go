// Package profile measures how near a rule's answers sit to its limit, so a
// rule author can see which rules are guessing without keeping hand labels.
package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hpcsc/vet/internal/questions"
	"github.com/hpcsc/vet/internal/verdict"
)

// Stat is what one rule did across every report.
//
// NearLimit counts answers the report already called unsure, which verdict
// decided with the same noulUnsureBand the text output uses. Profile does not
// hold a threshold of its own, so the two can never disagree about which
// answers were close to the line.
type Stat struct {
	Rule      string
	Type      questions.Kind
	Asked     int
	NearLimit int
	Reports   int
	values    []float64
}

// share is the part of a rule's answers that sat too near the limit to be a
// distinction the model really made. Only a noul rule has a limit to crowd, so
// a choice or score rule has no share and reports none rather than inventing
// one.
func (s Stat) share() (float64, bool) {
	if s.Asked == 0 || s.Type != questions.Noul {
		return 0, false
	}
	return float64(s.NearLimit) / float64(s.Asked), true
}

// median is the middle answer for a noul rule, and the middle confidence for a
// choice rule. A score has no single number, so it has no median.
func (s Stat) median() (float64, bool) {
	if s.Type == questions.Score || len(s.values) == 0 {
		return 0, false
	}
	sorted := append([]float64(nil), s.values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle], true
	}
	return (sorted[middle-1] + sorted[middle]) / 2, true
}

// Summarize counts what every rule did. A rule that was never asked is absent,
// which is itself worth seeing: the table says nothing about a rule the change
// gave nothing to judge.
func Summarize(reports []verdict.Report) []Stat {
	byRule := map[string]*Stat{}
	order := []string{}
	for _, report := range reports {
		for _, group := range report.Groups {
			for _, row := range group.Answers {
				stat, ok := byRule[row.Rule]
				if !ok {
					stat = &Stat{Rule: row.Rule, Type: row.Type}
					byRule[row.Rule] = stat
					order = append(order, row.Rule)
				}
				if stat.Type == "" {
					stat.Type = row.Type
				}
				stat.Asked++
				if row.Unsure {
					stat.NearLimit++
				}
				if row.Violates {
					stat.Reports++
				}
				switch {
				case row.Type == questions.Noul:
					if v, ok := row.Value.(float64); ok {
						stat.values = append(stat.values, v)
					}
				case row.Type == questions.Choice && row.Confidence != nil:
					stat.values = append(stat.values, *row.Confidence)
				}
			}
		}
	}
	stats := make([]Stat, 0, len(order))
	for _, rule := range order {
		stats = append(stats, *byRule[rule])
	}
	// Worst first, so the rule to fix is the first thing read. Ties fall back
	// to how often the rule was asked, then to the name, so two runs of the
	// same reports print the same table.
	sort.Slice(stats, func(i, j int) bool {
		a, aok := stats[i].share()
		b, bok := stats[j].share()
		switch {
		case aok != bok:
			return aok
		case aok && a != b:
			return a > b
		case stats[i].Asked != stats[j].Asked:
			return stats[i].Asked > stats[j].Asked
		default:
			return stats[i].Rule < stats[j].Rule
		}
	})
	return stats
}

// Write prints one row per rule.
func Write(w io.Writer, stats []Stat) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "rule\ttype\tasked\tnear limit\treports\tmedian")
	for _, s := range stats {
		near := "-"
		if share, ok := s.share(); ok {
			near = fmt.Sprintf("%d (%.0f%%)", s.NearLimit, share*100)
		}
		median := "-"
		if v, ok := s.median(); ok {
			median = fmt.Sprintf("%.2f", v)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%d\t%s\n", s.Rule, s.Type, s.Asked, near, s.Reports, median)
	}
	tw.Flush()
}

// Gate turns the table into something CI can fail on, so a rule that ships
// answering near its limit is caught before it reports noise on every commit.
func Gate(stats []Stat, maxShare float64) error {
	if maxShare <= 0 {
		return nil
	}
	var over []string
	for _, s := range stats {
		if share, ok := s.share(); ok && share > maxShare {
			over = append(over, fmt.Sprintf("%s (%.0f%%)", s.Rule, share*100))
		}
	}
	if len(over) == 0 {
		return nil
	}
	return fmt.Errorf("%s answer near the limit more than %.0f%% of the time", strings.Join(over, ", "), maxShare*100)
}

// Reports reads the reports a replay saved, which are the JSON the vet command
// already writes. Only the shape replay produces is read, so a stray file in
// the directory cannot change what profile reports.
func Reports(dir string) ([]verdict.Report, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", "1.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no reports under %s, run vet replay first", dir)
	}
	reports := make([]verdict.Report, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var report verdict.Report
		if err := json.Unmarshal(raw, &report); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}
