# benchpress design

benchpress keeps a repository's **workbench**: the topics its author and agents
are working through — bugs seen in play, changes asked for, design questions,
ideas — one Markdown file per topic, with one ordered list of what comes next.
The files are the product. People and agents read and edit them with ordinary
tools; benchpress checks them and draws a board over them.

This document is binding. A change to the format, a lint rule or a key binding
changes this document in the same commit.

## 1. What benchpress is not

- **Not an agent tool.** It runs no agent, talks to no model, serves no MCP and
  hands agents no workflow beyond its one skill (§8). Orchestrating agents is
  ploopy's job.
- **Not a tracker.** No IDs, assignees, priorities, columns, milestones, due
  dates, comments or history. Git keeps history.
- **Not a committer.** It writes files and never runs a git command that
  changes anything. The author commits.
- No daemon, no web UI, no network.

The one contract with ploopy is a path: a topic taken by a pass names the
pass's plan file (§3, `plan`). Neither tool imports, calls or configures the
other.

## 2. Layout and configuration

A repository opts in with `.benchpress.toml` at its root:

```toml
dir = "docs/workbench"                           # where the topics live
kinds = ["bug", "change", "design", "idea"]      # allowed kinds, in display order
areas = ["economy", "maps", "army"]              # allowed areas, in display order
```

- Every key is optional. `dir` defaults to `docs/workbench`; `kinds` defaults to
  the list above. An empty or missing `areas` allows any area that is a slug
  (§3), displayed alphabetically.
- An unknown key is an error, so a misspelt key is not silently ignored.
- benchpress finds the repository root by walking up from the working directory
  to the first directory holding `.benchpress.toml`. Without one, every command
  but `init` and `skill` fails with `no .benchpress.toml here or above; run
  benchpress init`.

Inside `dir`:

- `NEXT.md` is the order (§4).
- `README.md`, if present, is ignored: a place to explain the workbench to
  readers browsing the repository.
- Every other `*.md` file directly in `dir` is a topic. Subdirectories and other
  files are ignored.

## 3. A topic file

The file name is the topic's **slug** and its only identity:
`^[a-z0-9]+(-[a-z0-9]+)*\.md$`, for example `drafts-strand.md`. Renaming a topic
is renaming the file. A slug need not match the title.

```markdown
---
kind: design
area: army
state: planned
plan: docs/plans/PLAYTEST_FIXES.md
---

# Drafts strand, and the player cannot help them

*Design, from the 2026-10-13 playtest.* What was said, dated.

**Now:** what the build does today, with landmarks.

**Open:**

- the questions still to settle
```

**Front matter.** The file's first line is `---`, and the front matter ends at
the next line that is exactly `---`. Every line between is `key: value`: the key
is one of the four below, followed by a colon and one space; the value runs to
the end of the line, trimmed, unquoted. No comments, no blank lines, no nesting,
no YAML beyond this. benchpress parses it by hand.

| key | required | value |
| --- | --- | --- |
| `kind` | yes | one of the configured kinds |
| `area` | yes | one of the configured areas, or any slug when none are configured |
| `state` | yes | `open`, `settled` or `planned` |
| `plan` | only when `state: planned` | a repository-relative path to the pass's plan file |

**States.** `open` — still being discussed. `settled` — decided, waiting for a
pass to take it. `planned` — a pass's plan has taken it. There is no done: a
finished or dropped topic is deleted, and git keeps it.

**Title.** The first non-blank line after the front matter is a level-one
heading, `# Title`, and it is the topic's only level-one heading. A level-one
heading with no text is not a title.

**Body.** Free Markdown. The skill (§8) gives the conventions — dated notes,
*Now*, *Open* — and benchpress does not check them.

**Links.** Topics link to each other with relative Markdown links,
`[Drafts strand](drafts-strand.md)`, optionally with a `#fragment`. Links with a
scheme (`https:`), absolute paths and bare fragments are not checked. Image
tokens, `![alt](target)`, are not links and are not checked, in a topic body or
in Next item text; an ordinary link beside an image is still checked. In a
label, a backslash escapes the next character — `\\`, `\[` and `\]` — and the
label ends at the first unescaped `]`, so escaped brackets do not close it.

**Editing.** When benchpress changes a topic it changes only the front-matter
lines it must — replacing a key's line in place, appending a new key just before
the closing `---`, or removing a line — and leaves every other byte of the file
as it was. Every write is atomic: a temporary file in the same directory, then a
rename, keeping the file's mode.

## 4. NEXT.md

The only place order lives.

```markdown
# Next

1. [Presentation on raised ground](raised-ground.md) — after playtest-fixes lands
2. [A graphics preset](graphics-preset.md)
3. [The war table](war-table.md)
```

- An **item** is a list line: `N. ` or `- ` at the start of the line. Each item
  begins with a link to a topic in `dir`, and may continue with free text after
  the link. Its label uses the escapes of §3. The target is a relative path that
  normalizes to a Markdown file
  directly in `dir`: `./a.md` names `a.md`, while `a`, `sub/a.md` and
  `../outside/a.md` do not. Lines that are not items — the heading, prose, blank
  lines — are kept as they are and not checked.
- A topic appears in Next at most once. An item whose target does not name a
  directly contained topic file is not a position for any topic, so it does not
  make a later valid item a duplicate.
- When benchpress rewrites Next (add, remove, move), it keeps every non-item
  line, keeps each item's text, writes all items as one contiguous numbered
  block (`1. `, `2. `, …) where the first item stood, and writes the file
  atomically. With no items yet, the block goes after the last line. A missing
  `NEXT.md` is created as `# Next\n\n` followed by the block.

## 5. Where a plan is

A `planned` topic's plan is **found** if the path names a regular file in the
working tree — a symlink to one counts — or lies in the tree of any local branch
(`refs/heads/*`). A plan run by ploopy often
lives only on its branch, in a worktree, until the branch lands; that is not a
problem, only a fact the board shows (`on raised-ground`).

benchpress asks git once per local branch, never once per topic and branch:
one `git for-each-ref --format=%(refname) refs/heads` and then, per branch, one
`git ls-tree -r -z --name-only <ref> -- <path>…` over every plan path in
question. It keeps full `refs/heads/…` refs internally and strips that prefix
only to report a branch's name; `-z` keeps unusual path bytes literal.

benchpress runs git with `LC_ALL=C` and keeps its stderr. Only git's
`fatal: not a git repository` diagnostic means the repository is absent; then
only the working tree is consulted, without an error. Every other git failure
is reported with its diagnostic instead of passing an absent-plan map off as
success.

## 6. Lint

`benchpress lint` reports every problem in the workbench, one per line, as
`path:line: error: message` or `path:line: warning: message`, with `path`
relative to the repository root and `line` 1-based (1 when the problem has no
better line). It exits 1 when there is any error and 0 otherwise. `--json`
prints `{"problems": [{"path", "line", "severity", "rule", "message"}]}` instead,
with the same exit code. Problems are sorted by path, then line.

Errors:

| rule | finding |
| --- | --- |
| `slug` | a topic's file name is not a slug |
| `front-matter` | the file does not start with `---`, the front matter is not closed, or a line in it is not `key: value` |
| `unknown-key` | a key other than `kind`, `area`, `state`, `plan` |
| `duplicate-key` | a key appears twice |
| `missing-key` | `kind`, `area` or `state` is missing |
| `kind` | the kind is not configured |
| `area` | the area is not configured, or is not a slug when none are |
| `state` | the state is not `open`, `settled` or `planned` |
| `plan-missing` | `state: planned` without `plan` |
| `plan-unexpected` | `plan` on a topic whose state is not `planned` |
| `plan-gone` | the plan is found neither in the working tree nor on any local branch: its pass has probably closed, so delete the topic or change its state |
| `title` | no level-one heading as the first content, a level-one heading with no text, or more than one |
| `next-item` | a Next item does not begin with a link to an existing topic |
| `next-duplicate` | a topic appears twice in Next |
| `topic-link` | a link from a topic or from Next to a `*.md` file in `dir` that does not exist |

Warnings:

| rule | finding |
| --- | --- |
| `link` | a relative link to a file outside `dir` that does not exist in the working tree |
| `next-absent` | `dir` has no `NEXT.md` |

The rule names are stable: they appear in `--json` and tests use them.

## 7. Commands

`benchpress` with no arguments opens the board (§9) when stdout is a terminal,
and runs `ls` otherwise. Every other command is non-interactive and prints
errors as `benchpress: message` with exit code 1.

| command | does |
| --- | --- |
| `init` | creates `.benchpress.toml` with the defaults, `dir` with `NEXT.md`, and the skill at `.agents/skills/benchpress/SKILL.md`; when `.claude/skills/` exists, also the symlink `.claude/skills/benchpress → ../../.agents/skills/benchpress`. Never overwrites an existing file; `--skill` rewrites the skill from the binary. Prints what it created. |
| `ls` | the Next items in order, then every other topic grouped by area (configured order, then alphabetical) and within an area by state (`open`, `settled`, `planned`) and then title. One line per topic: slug, state, kind, title, and for a planned topic its plan and where it was found. `--state`, `--kind`, `--area` filter, and each may be repeated. `--json` prints `{"next": [slug…], "topics": [{"slug", "path", "title", "kind", "area", "state", "plan", "plan_found": ["worktree" or a branch name…]}]}`, topics in the same order as the text. A topic that does not parse is listed, not skipped: by slug with its first parse error in the text, and with whichever of the JSON fields did parse. |
| `new <title> --kind K --area A` | creates `<slug>.md` from the title (lower case; every run of characters outside `a-z0-9` becomes one hyphen; leading and trailing hyphens dropped), with state `open`, the title and the empty body sections `**Now:**` and `**Open:**`. Fails if the file exists or the kind or area is not allowed. Prints the path. `--next` also appends it to Next. |
| `state <slug> <state> [--plan PATH]` | sets the state. `planned` requires `--plan`; any other state removes `plan`. |
| `next add <slug> [--at N]` | adds the topic to Next, at the end or at position N (1-based). |
| `next rm <slug>` | removes it from Next. |
| `next mv <slug> <N>` | moves it to position N. |
| `rm <slug>` | deletes the topic file, removes it from Next, and prints the topics that still link to it. |
| `lint [--json]` | §6. |
| `skill` | prints the embedded skill. |

Commands that change files print nothing on success beyond what the table says.

## 8. The skill

`skills/benchpress/SKILL.md` in this repository is embedded in the binary and
installed by `init`. It tells an agent how to write and keep a workbench: what
a topic holds, the states, when to delete, that Next is the author's order, and
to run `benchpress lint` after editing. It is the format's documentation for
agents; this document is its documentation for benchpress's own developers.

## 9. The board

A full-screen terminal view, `benchpress` alone.

**Layout.** Two panes side by side. The left is the list: a *Next* section with
its items numbered, then the other topics grouped as `ls` groups them, each
group under its name. A row shows a state mark (`○` open, `◆` settled, `▶`
planned), the kind, the title, and for a planned topic the branch its plan is on
when it is not in the working tree. The right pane shows the selected topic's
body rendered as Markdown, front matter left out. A status line at the bottom
shows the lint summary (`2 errors, 1 warning`, or `lint clean`), the active
filter and grouping, and a one-line hint of the keys. Rows with a lint error are
marked. Below 100 columns the preview pane is hidden and `enter` toggles between
list and preview.

**Keys.**

| key | does |
| --- | --- |
| `j` `k` `↓` `↑` | move the selection |
| `g` `G` | first and last row |
| `ctrl+d` `ctrl+u` | scroll the preview |
| `/` | filter by text over title, slug, kind and area; `esc` clears |
| `tab` | cycle the grouping: area, state, kind |
| `s` | cycle the state open → settled → planned → open; reaching planned asks for the plan path, prefilled with `docs/plans/`, and `esc` cancels |
| `n` | add the topic to the end of Next, or remove it if it is there |
| `J` `K` | move a Next item down or up |
| `e` `enter` | open the topic in `$VISUAL`, else `$EDITOR`, else `vi`, and reload when the editor exits (`enter` toggles the preview instead below 100 columns) |
| `a` | add a topic: asks for title, kind and area |
| `d` | delete the topic after a `y` confirmation |
| `r` | reload from disk |
| `?` | the full key list |
| `q` `ctrl+c` | quit |

Every change goes through the same operations as the commands in §7 and is
written to disk at once; the board then reloads and re-runs lint. A failed
operation shows its error in the status line and changes nothing.

**Reloading.** Once `board.Load` has succeeded, the reload publishes that board,
its bodies, rows and preview even if the plan lookup or lint then fails. Plan
locations and lint results that a failed lookup leaves unavailable are cleared
rather than shown as current, and the status line says `lint unavailable` in
place of a lint summary until a reload completes both the plan lookup and lint.
A successful change followed by a refresh error reports `change saved; reload
failed: …`; an explicit reload reports `reload failed: …`. A read failure
before any new board is available keeps the previous board and reports the
refresh failure.

Every reload re-reads `.benchpress.toml` at the board's established root,
never rediscovering a different root. A valid configuration replaces the
previous one, including `dir`, `kinds` and `areas`; a changed directory is
loaded, later writes go there, and the selection stays on the same slug when
the new workbench still has it. An invalid configuration keeps the last
coherent configuration and reports the refresh error. When a change has already
been written, the reload still loads the disk with that last coherent
configuration so the change is shown, and reports the configuration error.

## 10. Packages

```
cmd/benchpress     cobra commands; the only package that prints or exits
embed.go           package benchpress: the embedded skill
internal/config    .benchpress.toml: defaults, unknown-key check, root discovery
internal/topic     one topic file: parse, title, links, front-matter edits. Pure text.
internal/next      NEXT.md: parse items, rewrite the block. Pure text.
internal/board     the directory: load every topic and Next, the ls order, the
                   operations of §7 (new, state, next, rm) as file writes, and
                   whether a repository-relative path exists
internal/repo      read-only git: the local branches and which plan paths each holds
internal/lint      the rules of §6 over a loaded board, a plan locator and an
                   existence query
internal/tui       the board of §9: bubbletea, bubbles, lipgloss, glamour
```

- `topic` and `next` do no file I/O beyond what their callers hand them: they
  take and return bytes.
- `lint` takes the plan locator as an interface and an explicit
  repository-relative existence query from its caller, so its tests need
  neither git nor a filesystem. The board answers the existence query.
- `repo` never runs a git command that writes.
- `tui` calls `board` and `lint`; it never edits a file itself. It takes a
  `repo.Locator` to find plans on reload, and reads `.benchpress.toml` through
  `config` on every reload at the board's established root, without writing it.
- Dependencies stay few: cobra, BurntSushi/toml, and charmbracelet's bubbletea,
  bubbles, lipgloss and glamour. No YAML library, no file watcher.
