# Proposal: an `audit profile` subcommand

## Status

A proposal, not a shipped command. The tool that produced every number below is
gitignored, and so is the sample of commits it replays, so none of this can be
re-run from a clean checkout. The figures are here because they are the argument
for the command, not because they are reproducible — anyone picking this up
should re-measure before believing them, and should read
[writing-questions.md](writing-questions.md) for the two experiments the numbers
come from.

## What it answers

One question: **is this rule decidable at all?**

`vet` asks a model to judge each rule against one file's path and diff. For some
rules that is enough. For others the deciding fact is not in the prompt, and the
model has no way to know that — so it returns a value near the threshold and the
report becomes a coin flip.

`profile` measures that directly, per rule, from runs that already exist. It
calls no model, so it is free to run on every commit.

This is deliberately separate from "were the reports right". That question needs
hand labels, needs maintenance, and only covers the handful of violations anyone
thought to label. Conditioning needs neither, and it covers every answer.

## The evidence for it

Measured on 9 commits, 183 answers, 27 rules, by replaying the sample and
reading the two resulting run directories back:

| Rule | asked | near limit, as written | near limit, after the rewrite |
| --- | --- | --- | --- |
| `assertion-strictness-mismatch` | 7 | 1 (14%) | 7 (100%) |
| `test-file-name` | 15 | 2 (13%) | 14 (93%) |
| `generic-implementation-name` | 13 | 1 (8%) | 12 (92%) |
| `rejected-operation-inert` | 5 | 0 (0%) | 3 (60%) |
| `comment-not-one-sentence` | 9 | 3 (33%) | 5 (56%) |
| `comment-carried-by-code` | 9 | 3 (33%) | 4 (44%) |

The right-hand column is the evidence-framing rewrite applied to all 27 rules,
which turned 9 reports and 5 correct into 46 reports and 7 correct. Conditioning
fell out long before anyone looked at correctness, and this ordering is the
point: the rewrite made seven rules guess, which is why it made more noise.

The rules as written are mostly well conditioned. That is not an argument that
conditioning is a poor signal — it is the reason the signal is worth having, a
rule that guesses looks exactly like a rule that is working until you check.

`test-file-name` is the clearest case. It asks whether
`internal/diff/loader_test.go` is named for the file it tests. The prompt
carries the file path and that file's diff
(`internal/backend/jev/client.go:88`) and nothing else — the model is never told
`internal/diff/file.go` exists. Under the rewrite, fourteen of fifteen answers
land on the threshold and twelve of those become reports. None of them are
findings.

The same distribution explains the rest of the accuracy story: answers churning
51% between two replays of one commit, and correct reports whose values overlap
wrong reports' values (0.50–0.55 against 0.50–0.76), which is why no threshold
tuning separates them.

## Proposed output

```
$ audit profile /tmp/runs
rule                            type    asked  near limit  reports  median
comment-carried-by-code         noul    9      3 (33%)     1        0.28
comment-not-one-sentence        noul    9      3 (33%)     1        0.16
file-named-for-type             noul    4      1 (25%)     0        0.34
test-file-name                  noul    15     2 (13%)     1        0.27
exposed-for-tests               noul    12     0 (0%)      0        0.04
struct-naming                   choice  3      -           1        0.60
naming-quality                  score   13     -           0        -
```

Worst first, so the author sees what to fix before anything else.

- **asked** — answers recorded for the rule. A rule absent from the table was
  never asked, which is itself worth seeing: the change gave it nothing.
- **near limit** — count and share of answers the report already called unsure.
  `verdict` decides that with the same `noulUnsureBand` the text output uses, so
  the two can never disagree about which answers were close to the line. A rule
  sitting here is guessing.
- **reports** — answers that crossed the limit.
- **median** — the middle value for a `noul` rule, the middle confidence for a
  `choice` rule. A rule sitting at 0.52 is telling you something different from
  one at 0.02, and the two render identically in the report. A `score` has no
  single number, so it has no median.

A `choice` rule reports no near-limit figure. It has no limit to crowd, and
picking a confidence band to measure against would be a number with nothing
behind it. Its median confidence is the honest part: `struct-naming` answering
at 0.60 is visibly less sure than one answering at 0.99.

## Two properties worth having

**A gate, not just a report.** `--max-near-limit 0.2` exits non-zero when any
rule exceeds the share. A new rule that ships at 80% near-limit then fails CI
instead of shipping noise on every commit.

**A falsifiable loop for authors.** The intended use:

```
edit a rule  ->  replay the sample  ->  audit profile  ->  near-limit falls?
```

If it does not fall, the fix was wrong or insufficient, and you know in seconds
rather than after a labelled audit. A rule author can check their own work
without maintaining a single hand label.

## Scope

Deliberately small:

- Reads saved run files. No model calls, no network, no git.
- Reuses `verdict.Row.Unsure`, so it does not introduce a second threshold for
  the same idea, and the saved-answer format already in the report.
- No per-rule needs, no thresholds per rule, no configuration.
- `choice` conditioning included from the start as median confidence; `score`
  rules report no near-limit figure, because a score has no threshold to crowd.

## Open questions

1. **Which run files.** Condition drifts between commits, so a single old run
   directory goes stale. The tool profiles every report under a directory, which
   means the author profiles a directory they just produced.
2. **Whether to keep the `median` column** once the table is in a terminal. It
   is the most useful number when reading one rule and the least when scanning
   twenty.
3. **Sample size.** Nine commits leave several rules asked once or twice, which
   is not enough to profile them. A rule with a low share because it was asked
   once is not a rule that is well conditioned.
