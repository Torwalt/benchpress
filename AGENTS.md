# AGENTS.md

Shared guidance for AI coding agents working in this repo. This file is
committed and is the source of truth; `CLAUDE.md` just imports it.

## What this project is

benchpress keeps a repository's workbench: one Markdown file per topic, an
ordered Next list, a lint over both and a terminal board over them. The files
are the product; the tool checks and shows them. It complements ploopy, which
runs plans, and knows nothing of it beyond a plan file's path.

`docs/DESIGN.md` is binding: the file format, the lint rules, the commands, the
board's keys and the package boundaries. A change to any of them changes that
document in the same commit.

## Rules

- benchpress never runs an agent, never talks to a model or the network, and
  never runs a git command that writes. It writes only files inside the
  workbench directory, `.benchpress.toml` and the installed skill.
- Edit a topic file surgically: only the front-matter lines that must change,
  every other byte kept, written atomically.
- Interactive is the primary interface: `benchpress` alone opens the board.
  Every board action has a command, so agents and scripts never need the board.
- Keep the dependency set small: cobra, BurntSushi/toml, and charmbracelet
  (bubbletea, bubbles, lipgloss, glamour). No YAML library, no file watcher, no
  logging framework.
- Prefer simple, explicit code; no speculative abstractions. Comments explain
  *why*, not *what* — sparse and terse, one fact per sentence, no narration.
- Commits: a short subject stating the change, an optional body of at most
  three bullets. No AI attribution, no `Co-authored-by`, no plan or unit names.

## Tests

Pure packages (`topic`, `next`, `lint`) are tested on bytes and in-memory
values. `board` is tested on directories under `t.TempDir()`. `repo` is tested
against real git repositories in temporary directories, with `HOME` pointed at
the temporary directory. The board's model is tested by sending it messages,
never by driving a terminal.

## Verification

```
scripts/verify            # build, vet, gofumpt
go test ./...             # unit tests
nix build .#benchpress    # the package; needs every new file staged
nix flake check           # sandboxed build + tests
```

`flake.nix` pins `vendorHash`. A change to `go.mod` or `go.sum` updates it in
the same commit: set it to `pkgs.lib.fakeHash`, `git add -A`, run `nix build
.#benchpress`, and copy the `got:` hash it reports. A flake sees only files git
tracks, so stage new files before any `nix` command.
