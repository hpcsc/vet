package questions

func Default() (File, error) {
	return Parse([]byte(defaultFile))
}

const defaultFile = `version: 1
rules:
  - id: no-flag-field
    instructions: |
      Does the change add a flag or knob that toggles behaviour?
    type: noul
    noulLimit: 0.5

  - id: touches-database
    instructions: |
      Which option best describes how the change touches the database?
    type: choice
    choices:
      no-db: The change does not touch the database.
      uses-db: The change reads or writes the database.
      migrates: The change alters the schema.
    violatesWhen: migrates

  - id: follows-logging-guideline
    instructions: |
      Rate how well the change follows the logging guideline.
    type: score
    scores:
      - Logs with slog
      - Logs directly to stdout
      - Adds prohibited logging
    scoreLimit: 2
`
