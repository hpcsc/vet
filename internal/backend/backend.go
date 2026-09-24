package backend

import (
	"context"

	"github.com/hpcsc/vet/internal/questions"
)

type State struct {
	Path string
	Diff string
}

type Answer struct {
	Rule       string
	Noul       *float64
	Choice     *string
	Score      *int
	Confidence *float64
}

type Backend interface {
	Ask(ctx context.Context, state State, questions questions.File) ([]Answer, error)
}

var _ Backend = (*Fake)(nil)

type Fake struct {
	answers []Answer
	err     error
	states  []State
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

func (f *Fake) Ask(_ context.Context, state State, _ questions.File) ([]Answer, error) {
	f.states = append(f.states, state)
	return f.answers, f.err
}

func (f *Fake) States() []State {
	return f.states
}
