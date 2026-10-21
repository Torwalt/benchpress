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
Agent: claude
Model: opus
Effort: high

**Goal:** every finding confirmed or dropped, duplicates merged, each one
classed as a slip, a gap or a design question; a fix plan for every finding that
holds, with recommended design choices already applied; those choices and their
alternatives put to the author.

**Done when:** one commit touches only `docs/plans/FIRST_VERSION_FIXES.md` and
this document, this document ends with a *For the author* section, and the fix
plan, when there is one, lints clean.
