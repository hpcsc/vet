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