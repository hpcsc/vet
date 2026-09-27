package questions

import "strings"

// Include names a piece of the repository to put in the prompt alongside the
// file's own path and diff.
//
// A rule can be undecidable not because it is badly worded but because the fact
// it turns on is not in the prompt. `test-file-name` asks whether
// `loader_test.go` is named for the file it tests, and the model is never told
// that file exists, so it has to guess. This is how a rule author says which
// fact to supply instead of trying to word around its absence.
type Include string

const (
	// SiblingFilePaths lists the other files in the same directory.
	SiblingFilePaths Include = "siblingFilePaths"
	// ContainingDirDeclarations gives the declarations in the file's directory,
	// which is how a rule that compares a name against a local set of types
	// gets that set without the body of every file.
	ContainingDirDeclarations Include = "containingDirDeclarations"
	// ContainingDirContent gives the full text of the file's directory, the
	// current file excepted.
	ContainingDirContent Include = "containingDirContent"
	// RepoDeclarations gives the declarations from every file in the
	// repository, which is how a rule that depends on a name never colliding
	// anywhere gets to check.
	RepoDeclarations Include = "repoDeclarations"
	// FileContent gives the whole current file, for a rule that turns on
	// something outside the changed lines.
	FileContent Include = "fileContent"
	// PreviousFileContent gives the file as it was before the change, for a
	// rule comparing the change against what was there.
	PreviousFileContent Include = "previousFileContent"
)

// includes is in the order the material is added to the prompt, not
// alphabetical. The order is fixed rather than the order the questions file
// listed so that the same set of rules produces the same prompt every run,
// which is what makes a saved run comparable to the next one.
var includes = []Include{
	SiblingFilePaths,
	ContainingDirDeclarations,
	ContainingDirContent,
	RepoDeclarations,
	FileContent,
	PreviousFileContent,
}

func (i Include) valid() bool {
	for _, known := range includes {
		if i == known {
			return true
		}
	}
	return false
}

func (i Include) String() string {
	return string(i)
}

// includeNames reads the known names in prompt order, for an error that has to
// tell the author what they could have written.
func includeNames() string {
	names := make([]string, len(includes))
	for i, include := range includes {
		names[i] = include.String()
	}
	return strings.Join(names, ", ")
}

// unionIncludes combines what several questions files asked for into one set,
// in prompt order and without repeats.
//
// One call judges every rule that applies to a file, so the prompt has to hold
// what all of them need. That makes the union of the set the right answer
// rather than a per-rule list, and it means two questions files cannot each
// pay for the same material twice.
func unionIncludes(groups ...[]Include) []Include {
	wanted := map[Include]struct{}{}
	for _, group := range groups {
		for _, include := range group {
			wanted[include] = struct{}{}
		}
	}
	var union []Include
	for _, include := range includes {
		if _, ok := wanted[include]; ok {
			union = append(union, include)
		}
	}
	return union
}
