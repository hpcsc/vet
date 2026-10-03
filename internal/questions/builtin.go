package questions

func Default() (File, error) {
	return Parse([]byte(builtinFile), "")
}

const builtinFile = `version: 1
name: Go change policy
# A rule must be decidable from the file path and diff the model sees, plus the
# material include names. A rule whose deciding fact is not in the prompt makes
# the model guess at the threshold instead of answer. See docs/writing-questions.md.
context: |
  This repository names a file for the type it declares, keeps a comment only
  where the code cannot carry the fact, and tests observable behavior through
  the public interface.
include: [siblingFilePaths]
rules:
  - id: file-named-for-type
    description: A file is not named for the type it declares.
    instructions: |
      Answer 1 only when the change adds a type declaration whose name is not
      the name of the file that holds it, in the repository's file-name style,
      or adds two unrelated types to one file. Answer 0 when every type the
      change adds is named for the file that holds it, and when the change adds
      no type declaration.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*.go"
    exclude:
      - "**/*_test.go"

  - id: interface-repeats-package
    description: An interface repeats its package name.
    instructions: |
      Answer 1 only when the change adds an interface whose name repeats its
      package name. Answer 0 when every interface the change adds does not
      repeat its package name, and when the change adds no interface.
    type: noul
    noulLimit: 0.5
    requiresAddedLine: '^\s*type\s+\w+\s+interface\b'

  - id: package-doc-comment
    description: The change adds a package or file doc comment.
    instructions: |
      Answer 1 only when the change adds a package or file doc comment. Answer
      0 when the change adds no such comment.
    type: noul
    noulLimit: 0.5
    requiresAddedLine: '^\s*//'

  - id: test-file-name
    description: A test file is not named for the file it tests.
    instructions: |
      Answer 1 only when the change renames or adds a test file whose name is
      not the file it tests plus _test, or names a test support file for its
      role instead of the type it declares. Answer 0 when every test file in the
      change is named for the file it tests, and when the change leaves the name
      alone.
    type: noul
    noulLimit: 0.5
    files:
      - "**/*_test.go"

  - id: test-double
    description: Which test double the change uses for a dependency.
    instructions: |
      Which test double does the change use for a dependency? Answer no-double
      when the change stubs no dependency out.
    type: choice
    choices:
      real: The real implementation or an in-memory double for the happy path.
      broken: A broken double that always fails, for error paths.
      recording: A recording double that captures call details.
      mock: A mock that verifies call sequences, the last resort.
      no-double: The change stubs no dependency out.
    violatesWhen: mock
    files:
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
