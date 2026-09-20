# GitHub CI controls

Branch pushes run source-quality checks. Pull-request checks are off by default;
add the `ci:full` label to request full PR validation. Main and version-tag pushes
run the full suite independently of PR labels.

## When checks run

| Event | Source quality + vulnerability scan | Container test/build | KIND levels 1–5 + rollout levels 3–5 |
|---|---|---|---|
| Push to any branch except `main` | Run | Off | Off |
| Push to `main`, including a completed PR merge | Run | Run | Run |
| Push a `v*` version tag | Run | Run, then package artifacts | Run |
| Open/update/reopen a PR without `ci:full` | Off | Off | Off |
| Add `ci:full`, or update/reopen a PR that has it | Run | Run | Run |
| Remove `ci:full` | Off; cancel obsolete PR work | Off | Off |
| Change an unrelated PR label | No new checks; existing work is unaffected | Same | Same |
| Manual branch run with `cluster_tests` unchecked | Run | Off | Off |
| Manual branch run with `cluster_tests` checked | Run | Run | Run |
| Manual run on `main` or a version tag | Run | Run | Run |

A small `policy` job selects the work and explains the selection in the run's
summary. An off PR therefore still has a short selection run, but no tool
installation, quality build, container build or cluster jobs. Unrelated label
events skip even that job. The policy is implemented in
[ci-policy.py](../scripts/ci-policy.py) and wired through
[ci.yaml](../.github/workflows/ci.yaml).

Enabled integration jobs print verified API checkpoints and add a per-level
PASS/FAIL summary. [API test logs](API-TEST-LOGS.md) explains the assertions,
expected rejections, retained response fields and downloadable reports.

Merging a PR into `main` updates `main` and invokes its push workflow. There is
no additional `pull_request: closed` trigger, avoiding a duplicate full run for
the same merge. Closing a PR without merging does not trigger full validation.
PR runs, when enabled, use the PR merge ref; the main push validates the resulting
main commit. These are different validation points.

## Enable or disable checks in the PR UI

1. If the label does not exist, a repository maintainer creates a repository
   label named exactly `ci:full` under **Issues → Labels → New label**. Suggested
   description: `Run full container, integration and rollout checks on this PR`.
2. Open the PR and choose **Labels** in its sidebar. Select `ci:full` to start
   full checks immediately. It remains enabled for later commits on that PR.
3. Remove `ci:full` to disable future PR checks and cancel the previous PR run
   through workflow concurrency. The new selection run reports the checks off.
4. To request another run without a new commit, remove and re-add `ci:full`, or
   use GitHub's rerun control for the desired enabled run.

These controls become active after the workflow and policy script are pushed to
GitHub. Label definitions and branch-protection settings are repository settings;
changing the local workflow does not create a label or change protection rules.
Fork PRs can still require a maintainer's normal Actions approval.

The PR label does not disable branch-push checks, main checks or version-tag
checks. Avoid GitHub's repository-wide **Disable workflow** control when the
intent is only to turn off checks for one PR.

## Manual run from Actions

After this workflow is present on the default branch, open **Actions →
development-ci → Run workflow**, choose the branch, and choose `cluster_tests`.
On a non-main branch, unchecked runs source checks; checked also runs the
containers and clusters. Main and version tags always select full validation.
Use the PR label when the desired result is attached to a PR merge-ref run.

## Cost, cancellation and required checks

Source checks retain formatting for all Go files, generated-code verification,
race tests, vet, four host builds, chart rendering, workflow/policy tests, module
verification and the vulnerability scan. The reduced branch path omits duplicate
container compilation/testing and the five Kubernetes jobs; it is not a weakened
formatting or unit-test path. Both paths use the pinned development tools.

New branch pushes replace older branch runs. PR updates and changes to `ci:full`
replace the prior PR run. Separate concurrency groups prevent branch pushes,
manual runs and unrelated label edits from canceling a PR run. Main/tag runs do
not cancel an already running main/tag run; GitHub's concurrency queue can still
replace an older pending run when several pushes arrive rapidly.

A branch push to a labeled PR can still run source checks in both contexts,
because the PR tests the merge ref. The expensive full matrix runs only on the
enabled PR, rather than on both branch push and PR. Direct main pushes, completed
main merges and version tags retain full validation.

Intentional job skips are not passing test evidence. GitHub treats skipped jobs
differently from workflows omitted by event/path filters. If repository rules
require full pre-merge tests, enabling optional PR checks is not itself an
enforcement policy: agree required checks and label permissions separately.
Existing protection settings have not been changed by this update.

## Validate changes locally

From the repository root in Linux/WSL:

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/dev.sh" workflow-check
```

This runs the event-policy regression tests and actionlint without creating a
cluster. The tests cover label add/remove/update, unrelated labels, main and
version-tag pushes, boolean/string manual inputs and GitHub output-file handling.
Full cluster behavior remains covered by the existing integration and rollout
targets; changing triggers does not constitute new hosted CI execution evidence.

Local verification on 2026-09-20: the three policy tests passed on Windows and
Ubuntu WSL, including 22 event-selection scenarios, unsupported-event rejection
and output/summary file handling. WSL `workflow-check` passed actionlint. The
delivery Mermaid/SVG/GIF assets were rebuilt and visually inspected; all six
offline guide pages passed the existing link/layout/playback checks. Hosted
label-trigger and cancellation behavior awaits a run after these changes are
pushed.

GitHub references: [workflow events](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows),
[concurrency controls](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency),
[required-check troubleshooting](https://docs.github.com/en/enterprise-cloud@latest/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks).
