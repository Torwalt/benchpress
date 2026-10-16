---
test: yes
---

# The first version of benchpress

Status: active task document, written 2026-10-16 against the repository's first
commit.

benchpress keeps a repository's workbench: one Markdown file per topic, an
ordered `NEXT.md`, a lint over both and a terminal board over them.
`docs/DESIGN.md` is the specification of everything this pass builds — the
format (§3, §4), where a plan is (§5), the lint rules (§6), the commands (§7),
the skill (§8), the board (§9) and the packages (§10). Units cite its sections
rather than restating them; when a unit and the design disagree, the design
wins, and a unit that must depart from it changes `docs/DESIGN.md` in the same
commit and says why in the handover.

## Decisions settled with the author (2026-10-16)

- **A tool of its own beside ploopy.** ploopy orchestrates agents; benchpress
  only organises topics. It runs no agent, serves no MCP, and commits nothing.
  The only contract between the two is a plan file's path in a topic.
- **The files are the product.** One Markdown file per topic named by its slug,
  minimal front matter (`kind`, `area`, `state`, `plan`), order only in
  `NEXT.md`, finished topics deleted rather than marked done.
- **Three states:** `open`, `settled`, `planned`.
- **A plan found on another local branch counts as found.** Fuller cross-branch
  views — showing topics deleted on a branch that has not landed — wait for a
  later version.
- **Generic, not tied to one repository:** the directory, kinds and areas are
  configuration.
- **The notes inbox is not benchpress's business.** Turning notes into topics is
  judgement, and lives in the skill.
- **Go, a TUI over the files, and a skill** telling agents how to write them.

## What is already here

The first commit holds the flake (with ploopy and the harnesses in its dev
shell), `go.mod` with cobra, `cmd/benchpress/main.go` as a root command with
`--version` and no subcommands, `embed.go` embedding
`skills/benchpress/SKILL.md` as `benchpress.Skill`, `scripts/verify`, and the
documents. `flake.nix` pins `vendorHash` for the current `go.sum`.

## Traps particular to this pass

- **`vendorHash`.** A unit that adds a dependency changes `go.sum`, and the
  flake's `vendorHash` with it, in the same commit, by the procedure in
  `AGENTS.md` *Verification*. `nix build .#benchpress` must succeed at the end
  of every unit that touched `go.mod`.
- **A flake sees only tracked files.** Stage new files (`git add -A`) before any
  `nix` command, or the build will not find them.
- **Git in tests.** Tests that run git set `HOME` to a temporary directory and
  configure `user.name` and `user.email` in the test repository, so they pass in
  the Nix sandbox and on a machine with no global identity.
- **Printing.** Only `cmd/benchpress` prints or exits; every other package
  returns values and errors.

## Stage 1 — The format

### 1.1 Configuration and root discovery

**Goal:** `internal/config` loads `.benchpress.toml` as `docs/DESIGN.md` §2
specifies: defaults for every key, an error naming any unknown key, and
`FindRoot(start string)` walking up to the first directory holding the file,
with the exact error message §2 gives when there is none. Add
`github.com/BurntSushi/toml` and update `vendorHash`.

**Start from:** `docs/DESIGN.md` §2; `toml.MetaData.Undecoded` reports unknown
keys.

**Done when:** `go test ./internal/config` covers the defaults, a full file, an
unknown key, discovery from a nested directory and from a directory with no
file above it; `scripts/verify` passes; `nix build .#benchpress` succeeds.

### 1.2 Topic files

**Goal:** `internal/topic` parses one topic file from bytes and edits its front
matter, as `docs/DESIGN.md` §3 specifies.

- `Parse(src []byte) (Topic, []Issue)`: the front-matter fields with their
  lines, the title and its line, and every relative link with its line. An
  `Issue` carries a rule name from §6, a line and a message, for the structural
  rules only: `front-matter`, `unknown-key`, `duplicate-key`, `missing-key`,
  `title`. Checking values against the configuration is lint's job, not this
  package's.
- Links: inline Markdown links `[text](target)` and `[text](target "title")`,
  ignoring fenced code blocks (```` ``` ```` and `~~~`) and inline code spans.
  Report the target without its `#fragment`; skip targets with a scheme, an
  absolute path, or only a fragment.
- `Set(src []byte, key, value string) ([]byte, error)` and
  `Remove(src []byte, key string) ([]byte, error)`, changing only the line
  concerned and leaving every other byte as it was (§3 *Editing*).
- `ValidSlug(name string) bool` and `Slugify(title string) string` (§7 `new`).

**Start from:** `docs/DESIGN.md` §3 and the `new` row of §7.

**Done when:** table tests cover a valid topic, each structural issue with its
line, links inside and outside code, `Set` replacing and appending, `Remove`, a
byte-for-byte round trip of everything outside the edited line, CRLF input left
CRLF, and `Slugify` on punctuation, digits and non-ASCII titles.

### 1.3 NEXT.md

**Goal:** `internal/next` parses `NEXT.md` into items and rewrites its list
block, as `docs/DESIGN.md` §4 specifies.

- `Parse(src []byte) Next`: each item's line, its link target (the first link,
  if the item begins with one) and the text after the link; the non-item lines
  kept as they were.
- Operations on the parsed value — add at the end or at a 1-based position,
  remove, move — and `Render() []byte` writing the items as one contiguous
  numbered block where the first item stood, every non-item line kept.
- `New() Next` for a missing file, rendering as `# Next\n\n` and the block.

**Start from:** `docs/DESIGN.md` §4.

**Done when:** tests cover bullets and numbers mixed, prose between items
gathered into place, an item that does not begin with a link (kept, with no
target), each operation including out-of-range positions (an error), a file
with no items, and a round trip of an unchanged file byte for byte.

## Stage 2 — The board on the command line

### 2.1 Where plans are

**Goal:** `internal/repo` answers, for a set of repository-relative paths, where
each is found: `worktree` when it exists on disk, and the name of every local
branch whose tree holds it, asking git once per branch as `docs/DESIGN.md` §5
specifies. Outside a git repository only the working tree counts, without an
error. Expose it behind the small interface `internal/lint` will take, defined
here or in `lint` as you judge best, so lint's tests can fake it.

**Start from:** `docs/DESIGN.md` §5; `os/exec` with a `context.Context`.

**Done when:** tests on real temporary git repositories cover a path in the
working tree only, on a second branch only, on both, on none, and a directory
that is not a repository; the git commands run are only `for-each-ref` and
`ls-tree`.

### 2.2 Loading the board, and `ls`

**Goal:** `internal/board` loads the workbench directory (§2: which files are
topics) into topics and Next, and orders them as `ls` does (§7). `benchpress ls`
prints them, with `--state`, `--kind`, `--area` and `--json` exactly as §7
specifies, plan locations from `internal/repo`. A topic with structural issues
is listed by slug with its first issue, not skipped.

**Start from:** `docs/DESIGN.md` §2 and the `ls` row of §7; the parsers from
`internal/topic` and `internal/next`.

**Done when:** board tests on a fixture directory cover the ordering (Next
first, then areas in configured order, then states, then title), README.md and
subdirectories ignored, and a broken topic listed; a command test runs `ls` and
`ls --json` against a fixture and checks the output; running `go run
./cmd/benchpress ls` in a fixture directory prints the list.

### 2.3 Lint

**Goal:** `internal/lint` checks a loaded board against the configuration and a
plan locator with every rule of `docs/DESIGN.md` §6 and nothing else, and
`benchpress lint` prints the problems and exits as §6 specifies, with `--json`.

**Start from:** `docs/DESIGN.md` §6; the structural issues `internal/topic`
already reports become lint problems with the topic's path.

**Done when:** one test per rule name in §6, each producing exactly that rule on
a minimal fixture and nothing else; a clean fixture produces no problems; the
ordering and the exit codes are tested through the command.

### 2.4 Changing the workbench from the command line

**Goal:** the operations of `docs/DESIGN.md` §7 — `new` (with `--next`),
`state`, `next add`, `next rm`, `next mv`, `rm` — as functions in
`internal/board` writing atomically, and the cobra commands over them.

**Start from:** `docs/DESIGN.md` §3 *Editing*, §4 and §7; `topic.Set` and
`topic.Remove`; the `next` operations.

**Done when:** tests cover each operation's success and its documented
failures (existing file on `new`, disallowed kind or area, `planned` without a
plan, an unknown slug, a position out of range); `rm` removes the topic from
Next and reports linking topics; after any operation `benchpress lint` on the
fixture reports no new error.

### 2.5 `init` and `skill`

**Goal:** `benchpress init` and `benchpress skill` as `docs/DESIGN.md` §7
specifies, installing `benchpress.Skill` from `embed.go`.

**Start from:** `docs/DESIGN.md` §7 and §8; `embed.go`.

**Done when:** tests cover `init` in an empty directory, a second `init`
changing nothing, `--skill` rewriting a changed skill, and the `.claude/skills`
symlink created only when that directory exists; `benchpress skill` prints the
embedded file.

## Stage 3 — The board

### 3.1 Looking at the board

**Goal:** `internal/tui` draws the board of `docs/DESIGN.md` §9 — the list with
its Next section and groups, the rendered preview, the status line with the lint
summary, the narrow layout below 100 columns — with the keys that change no
file: movement, `g` `G`, preview scrolling, `/` filtering, `tab` grouping, `?`
and quitting. `benchpress` with no arguments opens it when stdout is a terminal
and runs `ls` otherwise. Add bubbletea, bubbles, lipgloss and glamour (the
`github.com/charmbracelet/...` v1 import paths) and update `vendorHash`.

**Start from:** `docs/DESIGN.md` §9 and §10; `internal/board` and
`internal/lint` for the data.

**Done when:** model tests send key and window-size messages and check the
selection, the filter, the grouping, the narrow toggle and that the view names
the selected topic's title; `scripts/verify` passes; `nix build .#benchpress`
succeeds; the board opens on a fixture under `script -qc 'timeout 2
./benchpress' /dev/null` without a panic.

### 3.2 Acting from the board

**Goal:** the board's keys that change files — `s` (with the plan prompt), `n`,
`J` `K`, `e`/`enter`, `a` (title, kind and area prompts with bubbles
`textinput`), `d` (with confirmation) and `r` — each through the
`internal/board` operations, then reloading and re-running lint, with a failed
operation's error in the status line, as `docs/DESIGN.md` §9 specifies. The
editor runs through `tea.ExecProcess`.

**Start from:** `docs/DESIGN.md` §9; the operations from `internal/board`.

**Done when:** model tests on a fixture directory cover each key's effect on
disk, the plan prompt's cancel, the delete confirmation's refusal, and an
operation's failure shown without a change; the editor key is tested with
`EDITOR` set to a script that appends a line.

## Stage 4 — Close the pass

### 4.1 Package, dogfood and document

**Goal:** the first version is installable and in use here.

- `nix flake check` passes.
- Run `benchpress init` in this repository and record, as an `open` topic, the
  cross-branch view the author deferred: showing topics a branch that has not
  landed has deleted or changed (see *Decisions settled with the author*).
  `benchpress lint` is clean.
- `README.md` gains a short *Use* section: `init`, the board, `lint` in a
  repository's checks, and how a pass's plan and a topic meet.
- `docs/DESIGN.md` describes what was built; correct anything that drifted
  during the pass.

**Start from:** `README.md`, `docs/DESIGN.md`, `flake.nix`.

**Done when:** `nix flake check` and `scripts/verify` pass; `benchpress lint`
exits 0 in this repository; `nix run . -- ls` lists the new topic.
