# Pull Requests

Use when opening a pull request or handling its review comments, including Codex's automated review.

## Opening a Pull Request

Open a pull request only when explicitly asked; a push alone does not authorize one.

- Use `gh` with the `runkids` account: `gh auth switch --user runkids` before any PR, issue, or review call.
- Branch as `runkids/<topic>` from the current commit (`git switch -c`), which keeps the working tree as it is.
- Before each commit, run `git diff --cached --name-only` and stage only task-owned files. Changes another session left in the tree stay unstaged unless the user asks to include them; name any you do include in the PR body.
- Fill `.github/PULL_REQUEST_TEMPLATE.md`. Leave a checklist box unticked when it is not true, and say why next to it.
- List the checks actually run, such as `make check`, UI tests and build, website build, or E2E runbooks. Name any check that was not run.
- Right after opening, start following Codex's first review yourself, as described below. Do not end the turn asking the user to report findings.

## Codex Review

Codex reviews a pull request when it is opened, and again for each `@codex review` comment. Findings arrive as inline review comments with a priority badge, such as P2. A summary comment, marked `codex-pull-request-review-summary`, shows the commit last reviewed.

Handle each finding as a bug report:

1. Read the cited code and confirm the scenario. A finding is a claim, not a fact.
2. If it holds, write a failing test that reproduces it, then fix it. Run `make check` in the devcontainer.
3. Commit the fixes as a `fix(...)` commit whose body explains each problem, and push to the pull request branch. Never amend or force-push commits already on the branch.
4. Reply in each thread in English: the fix commit, what changed, and the covering test. For a finding that does not hold, reply with the evidence instead. Do not resolve threads; the maintainer does.
5. Comment `@codex review` on the pull request to request a review of the new commit.

After opening the pull request, and after each `@codex review` comment, follow up until Codex answers. Poll in the background (for example every 30 seconds, up to 30 minutes). A 👀 reaction on the comment, or **Running** in the summary, means the review is still in progress. Codex has answered when any of these appears:

- a review submitted after the `@codex review` comment;
- the summary row showing **Completed** for the new commit;
- a 👍 reaction on that comment, which means no findings.

```sh
gh api repos/runkids/skillshare/pulls/<n>/comments -q '.[] | "\(.id) \(.path):\(.line) \(.body)"'
gh api repos/runkids/skillshare/pulls/<n>/comments/<id>/replies -f body='Fixed in <sha>. ...'
gh pr comment <n> --body '@codex review'
gh api repos/runkids/skillshare/pulls/<n>/reviews -q '.[] | "\(.submitted_at) \(.user.login) \(.state)"'
gh api repos/runkids/skillshare/issues/comments/<comment-id>/reactions -q '.[].content'
```

When new findings arrive, repeat from step 1 within the limits below. Report each round to the user: what was found, what changed, and which threads were answered.

## Review Rounds

A pull request should settle in one or two Codex rounds. Before opening it, and before every further `@codex review`, sweep every consumer of each concept the change adds or alters (CLI text, `--json`, TUI, server handlers, dashboard, status and doctor, docs, translations), run the realistic end-to-end scenarios, and fix everything found in one commit. For a change that mutates records or reconciles state, first list the state dimensions it crosses (global and project mode, plain and tracked installs, followed links, legacy keys, lock pins, audit acceptances, ambiguous matches) and cover each with a test before the first review; Codex finding them one per round means the matrix was never written down. One fix per round wastes the maintainer's time.

Not every finding deserves code. Fix a finding when a user can hit it through a supported workflow. When it needs an unrealistic combination (hand-edited metadata, a manifest pointed at another source while a byte-identical copy exists elsewhere, YAML syntax no real file uses), reply in the thread with the supported scope and why the case is outside it, add the case to the pull request's "Not covered" section, and do not request another review for it.

Stop requesting reviews, and report to the user, when any of these holds:

- Three rounds are done. From then on, answer new findings in the thread and in "Not covered" instead of changing code, unless a finding shows data loss or a broken supported workflow.
- A round produced no finding that changes behaviour.
- Fixing a finding would widen the change beyond the pull request's scope. Say so in the thread and leave it for a follow-up issue.

The user decides whether a fourth fixing round happens; do not start one on your own.
