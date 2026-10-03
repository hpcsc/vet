package backend

import (
	"context"
	"sync"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/material"
	"github.com/hpcsc/vet/internal/questions"
)

type Fake struct {
	mu      sync.Mutex
	answers []Answer
	err     error
	files   []diff.File
}

func NewFake() *Fake {
	return &Fake{}
}

func (f *Fake) WithAnswers(answers ...Answer) *Fake {
	f.answers = answers
	return f
}

func (f *Fake) WithError(err error) *Fake {
	f.err = err
	return f
}

func (f *Fake) Ask(_ context.Context, file diff.File, _ questions.File, _ []material.Section) ([]Answer, error) {
	f.mu.Lock()
	f.files = append(f.files, file)
	f.mu.Unlock()
	return f.answers, f.err
}

func (f *Fake) Files() []diff.File {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.files
}