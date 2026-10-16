# benchpress

A repository's workbench as plain Markdown: one file per topic — a bug, a
change, a design question, an idea — an ordered `NEXT.md`, a lint that keeps
them honest, and a terminal board to look at them and move them along.

It runs no agent and commits nothing. Agents edit the files with their own
tools, following the skill `benchpress init` installs; the author commits.
A topic a pass has taken names the pass's plan file, which is how it sits
beside [ploopy](https://github.com/Torwalt/ploopy).

The format, the rules and the keys are in [docs/DESIGN.md](docs/DESIGN.md).

## Development

```
nix develop        # go, the linters, ploopy and its harnesses
scripts/verify
go test ./...
```
