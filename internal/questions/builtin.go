package questions

func Default() (File, error) {
	return Parse([]byte(builtinFile), "")
}

const builtinFile = `version: 1
name: Go service policy
context: |
  This repository treats public behavior and stored data as contracts.
  Tests should prove what callers observe, not how the code is arranged internally.
rules:
  - id: tests-through-public-api
    description: The change's tests assert on implementation details.
    instructions: |
      The change's tests assert private fields, internal call counts, call
      order, or intermediate state instead of the result a caller observes.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"

  - id: rejected-operation-inert
    description: A rejected operation's test does not check the state stayed unchanged.
    instructions: |
      A test of a rejected operation checks the error but not that the
      observable state stayed unchanged where the public interface can show it.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"

  - id: comment-adds-guidance
    description: A comment repeats the code instead of explaining a constraint.
    instructions: |
      The change adds a comment that repeats what the code says, narrates the
      task, or leans on a ticket or document instead of explaining a
      constraint a reader needs.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*.go"

  - id: test-double
    description: Which test double the change uses for a dependency.
    instructions: |
      Which test double does the change use for a dependency?
    type: choice
    choices:
      real: The real implementation or an in-memory double for the happy path.
      broken: A broken double that always fails, for error paths.
      recording: A recording double that captures call details.
      mock: A mock that verifies call sequences, the last resort.
    violatesWhen: mock
    files:
      - "**/*_test.go"

  - id: public-contract-change
    description: The change preserves its public contract.
    instructions: |
      Which option best describes the compatibility of this change for
      existing callers, stored data, and integrations?
    type: choice
    choices:
      compatible: No existing caller or stored record needs to change.
      additive: The change adds behavior without changing existing behavior.
      breaking: The change removes or changes behavior that existing callers or stored data rely on.
    violatesWhen: breaking
    files:
      - "**/*.go"
    exclude:
      - "**/*_test.go"

  - id: testing-quality
    description: How well the change's tests follow the testing policy.
    instructions: |
      Rate how well the change's tests prove observable success and failure
      behavior and remain independent of production implementation details.
    type: score
    scores:
      - Tests prove observable behavior and cover the relevant success and failure paths.
      - The tests follow the policy with one minor gap.
      - A test locks in implementation details or cannot fail on a real defect.
    scoreLimit: 2
    files:
      - "**/*_test.go"
`
