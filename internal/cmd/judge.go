package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/hpcsc/vet/internal/verdict"
)

// exitCode is the exit status a run that finished asks the process to end
// with. It is a value, not an error message, so main does not print a vet:
// line for it.
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("exit code %d", int(c)) }

type judge struct {
	out       io.Writer
	repo      *git.Repo
	backend   backend.Judge
	questions string
	base      string
	json      bool
	exit      bool
}

func (j *judge) run(ctx context.Context) error {
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
	answers, err := j.askAll(ctx, q, files)
	if err != nil {
		return err
	}
	report, err := verdict.Judge(base, q, answers)
	if err != nil {
		return err
	}
	if err := j.render(report); err != nil {
		return err
	}
	if report.Violations > 0 && j.exit {
		return exitCode(1)
	}
	return nil
}

func (j *judge) askAll(ctx context.Context, file questions.File, files []diff.File) ([]verdict.FileAnswers, error) {
	sem := make(chan struct{}, 4)
	results := make([]verdict.FileAnswers, len(files))
	errs := make([]error, len(files))
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Add(1)
		go func(i int, f diff.File) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			answers, err := j.backend.Ask(ctx, f, file)
			results[i] = verdict.FileAnswers{Path: f.Path, Answers: answers}
			errs[i] = err
		}(i, f)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (j *judge) render(report verdict.Report) error {
	if j.json {
		enc := json.NewEncoder(j.out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	_, err := fmt.Fprintln(j.out, report.Text())
	return err
}
