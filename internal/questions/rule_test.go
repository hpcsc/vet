//go:build unit

package questions

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRule(t *testing.T) {
	parse := func(t *testing.T, yaml string) error {
		t.Helper()

		_, err := Parse([]byte(yaml))
		return err
	}

	t.Run("validate", func(t *testing.T) {
		t.Run("a rule with no id is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - type: noul\n    noulLimit: 0.5")

			require.ErrorContains(t, err, "id")
		})

		t.Run("a rule with empty instructions is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    type: noul\n    noulLimit: 0.5")

			require.ErrorContains(t, err, "instructions")
		})

		t.Run("a noulLimit below 0 is rejected", func(t *testing.T) {
			err := parse(t, noulRule(-0.1))

			require.ErrorContains(t, err, "noulLimit")
		})

		t.Run("a noulLimit above 1 is rejected", func(t *testing.T) {
			err := parse(t, noulRule(1.1))

			require.ErrorContains(t, err, "noulLimit")
		})

		t.Run("a violationsWhen that is not a choice key is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: what changed?\n    type: choice\n    choices:\n      no-db: no database\n    violatesWhen: schema")

			require.ErrorContains(t, err, "violatesWhen")
		})

		t.Run("a scoreLimit outside the scores range is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: how good?\n    type: score\n    scores:\n      - good\n      - bad\n    scoreLimit: 3")

			require.ErrorContains(t, err, "scoreLimit")
		})

		t.Run("an unknown type is rejected", func(t *testing.T) {
			err := parse(t, "version: 1\nrules:\n  - id: a-rule\n    instructions: what changed?\n    type: number")

			require.ErrorContains(t, err, "type")
		})
	})
}

func noulRule(limit float64) string {
	return fmt.Sprintf(`version: 1
rules:
  - id: a-rule
    instructions: does it?
    type: noul
    noulLimit: %g
`, limit)
}