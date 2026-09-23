//go:build unit

package backend

import (
	"context"
	"errors"
	"testing"

	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, text string) questions.File {
	t.Helper()

	file, err := questions.Parse([]byte(text))
	require.NoError(t, err)
	return file
}

func TestFake(t *testing.T) {
	questions := mustParse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: does it?\n    type: noul\n    noulLimit: 0.5\n")

	t.Run("ask", func(t *testing.T) {
		t.Run("records the state it was asked and returns the configured answers", func(t *testing.T) {
			limit := 0.2
			fake := NewFake().
				WithAnswers(Answer{Rule: "a-rule", Noul: &limit})

			answers, err := fake.Ask(context.Background(), State{Path: "a.go", Diff: "@@ -1 +1 @@"}, questions)

			require.NoError(t, err)
			require.Equal(t, []Answer{{Rule: "a-rule", Noul: &limit}}, answers)
			require.Equal(t, []State{{Path: "a.go", Diff: "@@ -1 +1 @@"}}, fake.States())
		})

		t.Run("returns the configured error", func(t *testing.T) {
			want := errors.New("the backend is down")
			fake := NewFake().WithError(want)

			_, err := fake.Ask(context.Background(), State{Path: "a.go", Diff: ""}, questions)

			require.ErrorIs(t, err, want)
		})
	})
}
