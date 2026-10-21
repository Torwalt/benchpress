---
reviews: docs/plans/FIRST_VERSION.md
test: no
---

# Review of FIRST_VERSION

Status: written by `ploopy review` on 2026-10-21 against `538cadc7`. It reviews
11 landed units of `docs/plans/FIRST_VERSION.md`.

Each collecting unit reads one landed unit's work order, handover and commits,
and writes its findings without changing the repository. The seams unit looks
only at what lies between the units. The triage unit weighs every finding,
writes the fix plan `docs/plans/FIRST_VERSION_FIXES.md` for every finding that
holds using its recommended design choices, and explains those choices and their
alternatives under *For the author*.

## Stage 1 — Collect

### 1.1 Review 1.1 Configuration and root discovery

Review: 1.1

**Goal:** findings on unit 1.1 of `docs/plans/FIRST_VERSION.md`, *Configuration
and root discovery*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.2 Review 1.2 Topic files

Review: 1.2

**Goal:** findings on unit 1.2 of `docs/plans/FIRST_VERSION.md`, *Topic files*,
against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.3 Review 1.3 NEXT.md

Review: 1.3

**Goal:** findings on unit 1.3 of `docs/plans/FIRST_VERSION.md`, *NEXT.md*,
against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.4 Review 2.1 Where plans are

Review: 2.1

**Goal:** findings on unit 2.1 of `docs/plans/FIRST_VERSION.md`, *Where plans
are*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.5 Review 2.2 Loading the board, and `ls`

Review: 2.2

**Goal:** findings on unit 2.2 of `docs/plans/FIRST_VERSION.md`, *Loading the
board, and `ls`*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.6 Review 2.3 Lint

Review: 2.3

**Goal:** findings on unit 2.3 of `docs/plans/FIRST_VERSION.md`, *Lint*, against
its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.7 Review 2.4 Changing the workbench from the command line

Review: 2.4

**Goal:** findings on unit 2.4 of `docs/plans/FIRST_VERSION.md`, *Changing the
workbench from the command line*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.8 Review 2.5 `init` and `skill`

Review: 2.5

**Goal:** findings on unit 2.5 of `docs/plans/FIRST_VERSION.md`, *`init` and
`skill`*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.9 Review 3.1 Looking at the board

Review: 3.1

**Goal:** findings on unit 3.1 of `docs/plans/FIRST_VERSION.md`, *Looking at the
board*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.10 Review 3.2 Acting from the board

Review: 3.2

**Goal:** findings on unit 3.2 of `docs/plans/FIRST_VERSION.md`, *Acting from
the board*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.11 Review 4.1 Package, dogfood and document

Review: 4.1

**Goal:** findings on unit 4.1 of `docs/plans/FIRST_VERSION.md`, *Package,
dogfood and document*, against its work order.

**Done when:** the findings file is written and the repository is unchanged.

### 1.12 Review the seams between the units

Review: seams

**Goal:** findings on what lies between the reviewed units: the contracts they
share, work one left for another, leftovers no unit owns, documents none
updated.

**Done when:** the findings file is written and the repository is unchanged.

## Stage 2 — Triage

### 2.1 Weigh the findings

Review: triage
Agent: codex
Model: gpt-6.1-sol
Effort: high

**Goal:** every finding confirmed or dropped, duplicates merged, each one
classed as a slip, a gap or a design question; a fix plan for every finding that
holds, with recommended design choices already applied; those choices and their
alternatives put to the author.

**Done when:** one commit touches only `docs/plans/FIRST_VERSION_FIXES.md` and
this document, this document ends with a *For the author* section, and the fix
plan, when there is one, lints clean.

## For the author

49 findings: 16 slips, 25 gaps and 6 design findings go to
`docs/plans/FIRST_VERSION_FIXES.md` in 44 units; 6 design recommendations are
explained below; 2 were dropped. These are the 49 distinct findings from 52
reports, after folding three duplicate pairs:

- 1.5 F1 joins 1.6 F1: Next target identity, fixed by 3.1 in the fix plan.
- 1.10 F2 joins 1.9 F2: modal quitting, fixed by 5.2.
- 1.11 F1 joins 1.10 F6: the undocumented TUI locator dependency, fixed by 3.3.

Missing assertions are classed as gaps even when their collectors called them
slips. They protect explicit behavior from the work order or design; they do
not establish that the current production implementation is wrong. Documentation
drift about the already chosen repo.Locator placement is a slip, not a new
architecture decision. The terminal predicate is likewise a local slip. Reload
failure handling and configuration refresh require the explicit choices below.
No finding remains unsure, and no additional finding was introduced.

### Should lint check image targets as links?

The topic link scanner currently walks past `!` and collects the following
image target as a link. This also affects Next because lint scans its item
text with the same reader. Missing assets can therefore produce link warnings
or topic-link errors. The order specifies inline links but does not settle an
image-validation policy; silently extending lint to assets is a design choice.

- **Recommended:** exclude image tokens from both entry points and document
  that limit. D1 preserves the requested link lint without adding an asset rule.
- **Alternatives:** include images under the existing link rules, documenting
  and testing that broader meaning; or add a separate image rule, which gives
  assets an explicit severity and scope but expands the stable lint contract.
- **Planned as:** D1, fix unit 1.4.

### How should plan lookup distinguish being outside git from git failing?

For a nonempty set of plan paths, Find tries branch enumeration and discards
every enumeration error, including corrupt refs and ownership refusals. Empty
path sets do not invoke git. The author requires worktree-only operation outside
git, while the original unit permits only for-each-ref and ls-tree. Suppressing
all failures turns repository trouble into false plan-gone reports.

- **Recommended:** retain those two command families, pin git's diagnostic
  locale to C, and recognize only its explicit not-a-repository fatal diagnostic
  as fallback. Return other errors with stderr. D2 preserves both constraints;
  its reliance on diagnostic wording is a deliberate, tested tradeoff.
- **Alternatives:** permit an additional read-only repository probe, revising
  the original command restriction and paying for another command; or detect
  repository metadata by walking the filesystem, which must account for linked
  worktrees, bare repositories and git environment overrides. Retaining blanket
  suppression would preserve today's false absence reports and is not recommended.
- **Planned as:** D2, fix unit 2.2. Unit 2.1 separately corrects literal path
  decoding and branch identities and updates the command forms in DESIGN §5.

### Where should lint obtain working-tree file existence?

Lint currently calls os.Stat and its package tests create temporary workbenches.
That contradicts AGENTS's explicit pure-package testing rule. Link checks and
the missing-Next check need an existence fact, but making that an implicit
filesystem read changes lint's package boundary. The current boundary document
also omits the TUI's already implemented repo.Locator dependency.

- **Recommended:** pass an explicit existence callback to lint.Check, implement
  the disk-backed query on Board, and test lint with in-memory boards and path
  maps. D3 also records the retained repo.Locator dependency; its placement was
  already permitted by the original plan.
- **Alternatives:** supply a complete file-set snapshot, requiring the loader
  to gather arbitrary external link targets; or explicitly relax AGENTS and
  DESIGN to allow filesystem-backed lint and directory tests, giving up the
  existing pure-package rule. A general filesystem interface adds more surface
  than these existence checks need.
- **Planned as:** D3, fix unit 3.3, including all command and TUI callers and
  DESIGN §10. No separate dependency-relocation unit is needed.

### How should free-form titles be represented in generated Next links?

New with --next and NextAdd concatenate a title into Markdown link text.
The readers stop at the first closing bracket. A title such as `A [b](c.md)`
therefore generates an entry interpreted as pointing at c.md, which can pass
lint while referring to the wrong topic. Fixing only the writer with escapes
would still fail because both current link readers do not understand them.

- **Recommended:** escape backslashes and brackets in generated labels, and
  teach both readers to skip escaped characters when finding the label end.
  D4 keeps the title and readable label without adding a Markdown dependency.
- **Alternatives:** use the slug as the label when a title cannot be represented,
  reducing parser work at the cost of display text; reject such titles before
  any write, narrowing currently accepted input; or implement balanced-bracket
  Markdown labels, preserving raw labels but widening the parsing task.
- **Planned as:** D4, fix units 4.1 and 4.2, with syntax recorded in DESIGN §3
  and §4 and end-to-end target/removal assertions.

### What should remain visible when a saved change cannot be fully refreshed?

A successful operation writes disk before reload. Reload can then load the
new files but fail during plan enrichment, leaving the old board interactive.
This requires a nonnil locator and a planned topic with a nonempty path.
An ls-tree error or injected locator error reaches it today; for-each-ref errors
will also reach it after unit 2.2. The status currently obscures that the write
succeeded, and another state key can act on stale data.

- **Recommended:** publish successfully loaded file data independently, clear
  unavailable derived locations and lint results, and explicitly show lint as
  unavailable. D5 distinguishes `change saved; reload failed` from operation
  failure, so the user can trust the displayed state without trusting stale lint.
- **Alternatives:** keep the previous complete snapshot but disable writes
  until a refresh succeeds, preserving derived consistency at the cost of
  blocking work; or close the board on enrichment failure, avoiding stale actions
  but losing the interactive session. Rolling back a valid write because git
  lookup failed adds a second operation and another failure opportunity.
- **Planned as:** D5, fix unit 5.6, including status, summary, preview and later
  recovery behavior, documented in DESIGN §9.

### Does reload include the workbench configuration?

The command line loads .benchpress.toml each time, but every board reload
reuses its startup Config pointer. After an external edit, r still reads the
old directory and rejects newly allowed kinds or areas. Explicit reload,
post-operation reload and editor exit share this path. The scope of reload
therefore needs a rule, including what happens when the new config is invalid.

- **Recommended:** re-read configuration at the established root on every
  reload and load its configured directory. D6 retains the last coherent config
  and reports invalid edits; after a successful write it attempts a load using
  that config so D5 can still show the saved state.
- **Alternatives:** define configuration as startup-only and require restarting
  the board after edits, which is simpler but narrows reload; or refresh config
  only on explicit r, making manual reload comprehensive while leaving editor
  and post-operation refreshes temporarily inconsistent with command behavior.
- **Planned as:** D6, fix unit 5.7, after D5, with invalid-config tests and
  DESIGN §9/§10 changes recording the read-only config dependency.

### Dropped

- F2 from 1.11: the installed and embedded skill copies are currently identical;
  no work-order or design requirement mandates a synchronization check for a
  hypothetical later edit, so this is an additional maintenance preference.
- F4 from 1.12: Topic.Broken is an unused convenience method but currently
  agrees with FirstIssue and Entry.Broken; removing or routing through it is
  cleanup preference without an observed correctness or contract defect.
