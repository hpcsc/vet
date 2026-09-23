package backend

import (
	"context"

	"github.com/hpcsc/vet/internal/questions"
)

// State is one changed file that a backend judges.
type State struct {
	Path string
	Diff string
}

// Answer holds the verdict of one rule for one state. Exactly one of Noul,
// Choice or Score is set, matching the kind of the rule the answer is for.
// Confidence is set when the backend reports one; a noul answer has none.
type Answer struct {
	Rule       string
	Noul       *float64
	Choice     *string
	Score      *int
	Confidence *float64
}

// Backend answers the questions for one changed file.
type Backend interface {
	Ask(ctx context.Context, state State, questions questions.File) ([]Answer, error)
}

var _ Backend = (*Fake)(nil)

// Fake records the states it was asked and returns what WithAnswers or
// WithError configured. It implements the Backend contract for tests.
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
