package questions

func Default() (File, error) {
	return Parse([]byte(builtinFile), "")
}

const builtinFile = `version: 1
name: example
context: |
  These guidelines apply to every rule below.
  - The codebase logs with slog, never to stdout.
rules:
  - id: no-flag-field
    description: The change adds a flag or knob that toggles behaviour.
    instructions: |
      Does the change add a flag or knob that toggles behaviour?
    type: noul
    noulLimit: 0.5

  - id: touches-database
    description: How the change touches the database.
    instructions: |
      Which option best describes how the change touches the database?
    type: choice
    choices:
      no-db: The change does not touch the database.
      uses-db: The change reads or writes the database.
      migrates: The change alters the schema.
    violatesWhen: migrates

  - id: follows-logging-guideline
    description: How well the change follows the logging guideline.
    instructions: |
      Rate how well the change follows the logging guideline.
    type: score
    scores:
      - Logs with slog
      - Logs directly to stdout
      - Adds prohibited logging
    scoreLimit: 2
`