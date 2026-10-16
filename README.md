# benchpress

A repository's workbench as plain Markdown: one file per topic — a bug, a
change, a design question, an idea — an ordered `NEXT.md`, a lint that keeps
them honest, and a terminal board to look at them and move them along.

It runs no agent and commits nothing. Agents edit the files with their own
tools, following the skill `benchpress init` installs; the author commits.
A topic a pass has taken names the pass's plan file, which is how it sits
beside [ploopy](https://github.com/Torwalt/ploopy).

The format, the rules and the keys are in [docs/DESIGN.md](docs/DESIGN.md).

## Use

Initialise the workbench and install the skill:

```console
$ benchpress init
```

That writes `.benchpress.toml`, the workbench directory with its `NEXT.md`, and
the skill at `.agents/skills/benchpress/SKILL.md`, and never overwrites an
existing file.

Run `benchpress` with no arguments to open the board — the topics grouped, the
selected one rendered beside them. In a pipe or a script the same command runs
`benchpress ls`.

Put the lint in the repository's checks:

```console
$ benchpress lint
```

It exits 1 on any error, so it belongs in CI and in a pre-commit hook.

A pass takes a topic by naming its plan file,
`benchpress state <slug> planned --plan docs/plans/<PLAN>.md`. That path is the
whole contract with ploopy: the board shows the branch a plan is on when it is
not in the working tree, and `lint` reports a topic whose plan has gone as
`plan-gone`.

## Development

```
nix develop        # go, the linters, ploopy and its harnesses
scripts/verify
go test ./...
```
