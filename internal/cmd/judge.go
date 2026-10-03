package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/material"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/hpcsc/vet/internal/verdict"
)

// exitCode is the exit status a run that finished asks the process to end
// with. It is a value, not an error message, so main does not print a vet:
// line for it.
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("exit code %d", int(c)) }

type tuiRenderer func(io.Writer, verdict.Report, bool) error

type judge struct {
	out       io.Writer
	errOut    io.Writer
	repo      *git.Repo
	backend   backend.Judge
	questions string
	base      string
	output    outputMode
	all       bool
	exit      bool
	tui       tuiRenderer
	// materialBytes counts what the added repository material cost across every
	// file, so a run says what the evidence was worth in prompt bytes instead of
	// leaving it to be guessed at from the rules that asked for it.
	materialBytes atomic.Int64
}

func (j *judge) run(ctx context.Context) error {
	started := time.Now()
	base, err := j.repo.DetectBase(ctx, j.base)
	if err != nil {
		return err
	}
	q, err := questions.Load(j.questions)
	if err != nil {
		return err
	}
	loader := diff.NewLoader(j.repo)
	files, err := loader.Load(ctx, base)
	if err != nil {
		return err
	}
	answers, err := j.askAll(ctx, q, files, base)
	if err != nil {
		return err
	}
	if n := j.materialBytes.Load(); n > 0 {
		fmt.Fprintf(j.diagnostics(), "vet: added %d bytes of repository material to the prompts\n", n)
	}
	report, err := verdict.Judge(base, q, answers)
	if err != nil {
		return err
	}
	if err := j.render(report); err != nil {
		return err
	}
	fmt.Fprintf(j.diagnostics(), "vet: judged in %s\n", time.Since(started).Round(time.Millisecond))
	if report.Violations > 0 && j.exit {
		return exitCode(1)
	}
	return nil
}

func (j *judge) askAll(ctx context.Context, file questions.File, files []diff.File, base string) ([]backend.Answer, error) {
	resolver := material.NewResolver(j.repo)
	sem := make(chan struct{}, 4)
	results := make([][]backend.Answer, len(files))
	errs := make([]error, len(files))
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Add(1)
		go func(i int, f diff.File) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			scoped, ok := file.ForPath(f)
			if !ok {
				return
			}
			sections, err := resolver.Resolve(ctx, scoped.Include, f, base)
			if err == nil {
				j.materialBytes.Add(int64(material.Bytes(sections)))
			}
			var raw []backend.Answer
			if err == nil {
				raw, err = j.backend.Ask(ctx, f, scoped, sections)
			}
			if err == nil {
				answers := make([]backend.Answer, len(raw))
				for k, a := range raw {
					a.Path = f.Path
					answers[k] = a
				}
				results[i] = answers
			}
			errs[i] = err
		}(i, f)
	}
	wg.Wait()
	all := make([]backend.Answer, 0, len(files))
	for i, err := range errs {
		if err != nil {
			return nil, err
		}
		all = append(all, results[i]...)
	}
	return all, nil
}

// diagnostics is where a run says what it added to the prompts. It is not the
// report's channel: the byte count is a fact about the run, not about the code
// being judged, and neither output mode should have to carry it.
func (j *judge) diagnostics() io.Writer {
	if j.errOut == nil {
		return io.Discard
	}
	return j.errOut
}

func (j *judge) render(report verdict.Report) error {
	mode := j.output
	if mode == "" {
		mode = outputModeText
	}
	switch mode {
	case outputModeText:
		if !j.all {
			report = report.ViolationsOnly()
		}
		_, err := fmt.Fprintln(j.out, report.TextWithPassing())
		return err
	case outputModeJSON:
		if !j.all {
			report = report.ViolationsOnly()
		}
		enc := json.NewEncoder(j.out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case outputModeTUI:
		return j.renderTUI(report)
	default:
		return fmt.Errorf("unknown output mode %q: want text, json, or tui", mode)
	}
}

func (j *judge) renderTUI(report verdict.Report) error {
	if j.tui != nil {
		return j.tui(j.out, report, j.all)
	}
	return renderTUI(j.out, report, j.all)
}
