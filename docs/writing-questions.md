# Writing questions for Jev

A rule that reads well to a person can still be unanswerable for Jev. This
document is about the one property that decides whether a rule works, and it is
a property of the *rule's subject*, not of its wording.

## The finding

`vet` sends Jev one file's path and one file's diff, plus the guideline as
context (`internal/backend/jev/client.go:88`). Nothing else. The model cannot
see the rest of the repository, the other files in the directory, or any
history.

So every rule splits into two kinds:

| Kind | Example | Can Jev decide it? |
| --- | --- | --- |
| **Decidable** — the condition is a fact inside the diff | `file-named-for-type`: does the type name in the diff match the file path in the prompt? | Yes |
| **Undecidable** — the condition needs something outside the diff | `test-file-name`: is `internal/diff/loader_test.go` named for the file it tests? The model is never told `internal/diff/file.go` exists. | No |

For a decidable rule, Jev is accurate and decisive. For an undecidable rule, Jev
guesses — and the guess lands near 0.5, which is exactly `noulLimit`. That is
the whole source of the 51% answer churn and the ~50% precision. It is not a
wording problem.

## Measurement 1: a decidable rule, worded two ways

Rule: `file.go` declaring `type File` is the correct pairing, so the correct
answer is "no violation". Three replays per variant.

| Wording | Violation case | Correct pairing | Margin |
| --- | --- | --- | --- |
| Short and specific, asked alone | 0.86 – 0.91 | 0.47 – 0.57 (one false report) | 0.38 |
| Same, plus 60k chars of irrelevant guideline | 0.87 – 0.88 | — | 0.00 cost |
| Same, batched with 27 other rules | 0.82 – 0.85 | — | 0.04 cost |
| "Judge the type declarations the change adds…" | 0.77 – 0.83 | **0.78 — reported** | inverted |
| Evidence-framed (below) | 0.88 – 0.90 | 0.30 – 0.32 | **0.58** |

Three things fall out:

- **Context volume is not the problem.** 60k characters of guideline that had
  nothing to do with the question changed the answer by nothing. The guideline
  is always sent. Do not try to trim it.
- **Batching is not the problem.** 27 rules in one call instead of one cost
  0.04. One rule per call would multiply API cost by about six to buy noise.
- **Wording matters, but only for a decidable rule.** Evidence-framing a
  decidable rule moved it from reporting correct code at 0.78 to passing it at
  0.30. The margin went from inverted to 0.58.

## Measurement 2: the same treatment applied to all 27 rules

I then applied evidence-framing to every rule in the questions files, following
what looked like the lesson above. It made things much worse.

| | Reports | Correct | Precision | Agreement |
| --- | --- | --- | --- | --- |
| Original 27 rules | 9 | 5 | 55.6% | 96.2% |
| All 27 evidence-framed | 46 | 7 | **15.2%** | **78.1%** |

The extra reports were not findings. They were hedges that crossed the line:

| Rule | Answers within 0.45–0.60, original | Answers within 0.45–0.60, reworded |
| --- | --- | --- |
| `test-file-name` | 2 of 15, none above 0.5 | 12 of 15, 10 above 0.5 |
| `generic-implementation-name` | 1 of 13 | 12 of 13 |

Both of those are undecidable. The original wording let the model pattern-match
and answer low. "Answer 1 only if you can point at X" told it to go looking for
X; the search failed, and a failed search produced 0.5 rather than 0.

**The lesson from measurement 1 does not transfer to measurement 2.** The 0.58
margin was real, but it came from a rule whose subject was in the diff. Applied
to a rule whose subject is outside the diff, the same phrasing removes the
model's ability to answer 0 confidently and replaces it with a guess.

## Principle 1: check decidability before you write anything

Ask: *is every fact the condition needs present in the path and the diff for
this one file?*

If no, the rule cannot be fixed by wording. It has three options, in order of
preference:

1. **Give the model the missing facts.** This is a change to `vet`, not to the
   questions file: put sibling file names, or the declared interfaces, or the
   previous revision of the file into the prompt. Once they are in the prompt,
   the rule becomes decidable and the wording guidance below applies.
2. **Narrow the rule to the decidable part.** A test file named after a type
   that the same file declares is decidable. Whether it is named after the
   file it tests is not.
3. **Drop it.** A rule that cannot be decided is a rule that reports noise at
   the rate of a coin flip. That is worse than no rule, because it costs a
   reader's attention on every commit.

## Principle 2: for a decidable rule, license the negative

Once the subject is present in the prompt, a rule that describes a pattern to
look for leaves the model one useful move: find something resembling it. If the
rule never says that "nothing matched" is an answer, the model supplies one.
A bare probability makes this worse — the model does not know the number, so it
splits the difference, and 0.5 sits on `noulLimit`.

State the escape positively, and make the subject something that can be named:

> Answer 1 only if you can point at a type declaration the change adds whose
> name is not the name of the file that holds it, written in the
> repository's file-name style. Answer 0 when every type the change adds is
> named for the file that holds it. If the change adds no type declaration,
> answer 0.

Note the two `Answer 0` clauses: one for the subject being present and
correct, one for the subject being absent. Both are real answers and both need
naming.

## Principle 3: keep it short, and keep examples out

The reworded rules were longer than the originals, and every regression came
from a reworded rule. Length costs you two ways: the model has more to hold, and
a concrete example in the instructions becomes something to pattern-match
against. `test-file-name` went to 12 false reports after gaining the example
"questions_test.go in package questions tests questions.go" — the model found
near-misses to a pattern it had just been handed.

State the condition. Do not demonstrate it.

## Principle 4: choose the type by the shape of the answer

| The answer is | Use | Why |
| --- | --- | --- |
| a condition you can point at, decidable from the diff | `noul` with an explicit `Answer 0` | Measured 0.58 margin. The default. |
| a choice from a small fixed set | `choice` with `violatesWhen` | The model names what it saw. Measured 0.99 / 0.67 confidence. Use when you can enumerate the answers. |
| a judgement with no ground truth | `score` | An opinion. Do not expect a sharp margin, and do not use it to cover a `noul` rule you could have written. |

## Principle 5: measure the conditioning, not the counts

A rule that answers 0.49 and 0.51 on the same input is not a rule. Count the
answers that sit near the threshold:

Measure it per rule before you tune it. The tables in this document were
produced by replaying saved commits and counting answers that land near the
threshold; `vet profile` (and the `vet replay` that produces its inputs) is the
standing command for that, so a rule author can check their own rule without
keeping hand labels.

A rule where most answers land in 0.45–0.60 is undecidable, whatever its
wording. Take it out of the questions file until the prompt carries the facts
it needs. The near-limit count is how you find these without guessing: every one
of the five worst rules in the experiment above was visible as a cluster of
values near 0.5, before anyone looked at whether the reports were right.

## What `include` is worth

`include` is a working feature whose benefit is unproven, and those two facts
are worth keeping apart.

Replaying the same nine commits with and without it, over a configuration of 27
rules:

| | no `include` | with `include` |
| --- | --- | --- |
| answers | 183 | 183 |
| answers near the limit | 11 | 7 |
| reports | 7 | 5 |
| reports the hand labels call correct | 4 | 3 |
| material added per run | 0 | 38,675 bytes |

The near-limit count is the only figure that moved in a consistent direction,
and it moved by four answers. The report-level differences are not evidence: one
of the correct reports that disappeared also disappeared in a run where the
configuration was byte-identical, so churn alone accounts for it. Answers churn
substantially between replays of the same commit, and at 183 answers per run
that is larger than the effect being looked for.

So the table is not evidence that `include` fixes rules. The claim it supports
is weaker: a rule answering at the threshold is guessing, four fewer guesses is
four fewer coin flips, and the rule the feature was built for still answers
wrongly with the material it needed. The cost is real — 38,675 bytes against a
context that already runs to 74,402.

If you keep it, check it rather than trust it: profile a run with the material
and one without, and judge the near-limit column. Treat a difference of one or
two answers per rule as churn rather than as a result.

## Checklist

- [ ] Every fact the condition needs is in the file path or the diff.
- [ ] Every `noul` rule contains `Answer 0` for the subject-present case and
      for the subject-absent case.
- [ ] Every `noul` rule's subject can be pointed at on a line.
- [ ] No rule says "judge", "rate against the patterns above", or "assess".
- [ ] No `instructions` field works by example.
- [ ] Enumerable answer → `choice` with `violatesWhen`, not `score`.
- [ ] `files` / `exclude` exclude code the rule cannot be violated by.
- [ ] Two replays of the same commit produce the same reports.
- [ ] No rule's answers cluster near `noulLimit`.
