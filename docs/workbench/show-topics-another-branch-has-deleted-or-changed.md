---
kind: change
area: git
state: open
---

# Show topics another branch has deleted or changed

*Deferred, 2026-10-16, from the first version.* The designed cross-branch view is
narrower than it could be: an unlanded branch's deletions and edits to topics do
not show.

**Now:** `internal/board/board.go` lists every `*.md` in the configured `dir` as
read from disk, and `internal/repo/repo.go` answers only which local branches
hold a *plan path* (`refs/heads/*`, one `ls-tree` per branch). Neither consults a
branch's tree for the topic files themselves, so a topic deleted on a branch that
has not landed looks present here, and a topic it changed shows the working
tree's body.

**Open:**

- Should `ls` and the board mark a topic a local branch has deleted, or show it
  under the branch it survives on?
- Does the view cover changed bodies too, or only existence?
- Which branches count: every `refs/heads/*`, or only those the current branch
  has diverged from?
- What does it cost: a `git ls-tree` per branch over the whole workbench
  directory, beside §5's per-plan lookup?
