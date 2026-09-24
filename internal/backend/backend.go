package backend

import (
	"context"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/questions"
)

type Answer struct {
	Rule       string
	Noul       *float64
	Choice     *string
	Score      *int
	Confidence *float64
}

type Judge interface {
	Ask(ctx context.Context, file diff.File, q questions.File) ([]Answer, error)
}

var _ Judge = (*Fake)(nil)

type Fake struct {
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

func (f *Fake) Ask(_ context.Context, file diff.File, _ questions.File) ([]Answer, error) {
	f.files = append(f.files, file)
	return f.answers, f.err
}

func (f *Fake) Files() []diff.File {
	return f.files
}