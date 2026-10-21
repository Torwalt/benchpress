---
test: yes
context: AGENTS.md docs/DESIGN.md docs/plans/FIRST_VERSION.md
---

# Fixes from the review of FIRST_VERSION

Status: active task document, written 2026-10-21 against `076debf` by the triage
of `docs/plans/FIRST_VERSION_REVIEW.md`.

Each unit fixes a defect a review found and the triage confirmed. Any design
decisions below are the triage's recommendations. This plan implements them as
written unless the author revises the plan before running it. Finding references
use the collecting session number, then its finding number; they are evidence,
not additional context a unit needs to obtain.

## Recommended design decisions

### D1 — Images are outside the link lint

**Choice:** collect inline links, excluding Markdown image tokens, in both
topic bodies and Next item text. A real link beside an image is still checked.
State the exclusion in DESIGN §3; add no image-existence rule.

**Why:** the reviewed order specifies inline links and the design describes
links between topics. Treating every image as a link introduces an unstated
asset-validation policy. Excluding images keeps the promised lint scope small.

### D2 — Only an actual non-repository error permits worktree-only fallback

**Choice:** keep the two read-only git command families, `for-each-ref` and
`ls-tree`. Run git with `LC_ALL=C`, retain stderr, and recognize only git's
fatal diagnostic beginning `fatal: not a git repository` as the outside-repo
case. Propagate every other command failure with its useful stderr and wrapped
error, including cancellation and a missing executable. Do not add `rev-parse`.
Document this distinction in DESIGN §5.

**Why:** the author explicitly allowed worktree-only operation outside git,
and the original unit explicitly restricted the commands. This preserves both
decisions without hiding corruption or ownership failures. Diagnostic matching
is narrower than a repository probe but depends on git's English diagnostic;
pinning its locale and testing the classification makes that tradeoff explicit.

### D3 — Lint receives file existence from its caller

**Choice:** add an explicit repository-relative existence callback to
`lint.Check`, alongside its plan locator. Move the current root-relative
`os.Stat` behavior to a small method on `board.Board`; command and TUI callers
pass that method. Lint tests construct boards, parsed bytes and an existence
map in memory. Keep the current existence semantics, including treating stat
errors as absence; changing those semantics is outside this fix. Record the
callback and board query in DESIGN §10. Also record the existing TUI dependency
on `repo.Locator` there; keep that interface in `repo`.

**Why:** AGENTS explicitly calls lint pure. Supplying its environmental fact
keeps filesystem ownership in board and needs neither a filesystem framework
nor another package. The original plan already permitted the locator interface
to live in repo; documenting that existing choice needs no new architecture.

### D4 — Generated Next link labels use Markdown backslash escapes

**Choice:** escape backslashes, opening brackets and closing brackets in the
title used as a generated Next label. Teach the topic and Next link readers to
skip escaped characters while locating the label's closing bracket, so the
generated label always resolves to the outer link's intended slug. Preserve
the original topic title and ordinary labels. Specify this supported label
syntax in DESIGN §3 and §4; do not add a general Markdown parser dependency.

**Why:** free-form titles are already accepted. Rejecting punctuation would
narrow that contract, while a slug-only label would discard useful display
text. Escaping plus matching readers solves the wrong-topic case at its
producer and both consumers with a bounded syntax change.

### D5 — A failed enrichment must not hide successfully loaded file state

**Choice:** once `board.Load` succeeds, publish that board, bodies, rows and
preview even if plan lookup or lint subsequently fails. Clear unavailable plan
locations and lint results rather than displaying old results as current, and
show `lint unavailable` until a complete refresh succeeds. A successful write
followed by a refresh error reports `change saved; reload failed: ...`; an
explicit refresh reports `reload failed: ...`. A read failure before any new
board is available retains the previous board and reports the refresh failure.
Document these states in DESIGN §9.

**Why:** a write has already succeeded; retrying it against a stale state is
wrong. Keeping fresh file data with explicitly unavailable derived information
is more useful than suppressing the refresh, rolling back a valid change, or
presenting an unverified clean lint summary.

### D6 — Every board reload re-reads configuration at the established root

**Choice:** explicit `r`, post-operation reloads and editor-exit reloads read
`.benchpress.toml` at the existing board root before loading its configured
directory. Do not rediscover a different root. A valid configuration replaces
the old one, including kinds, areas and dir. On invalid configuration, retain
the last coherent configuration and report the refresh error. After a successful
operation, also attempt a disk load with that last configuration so the saved
change can still be shown under D5; never silently accept the invalid config.
Document config reloads and the TUI's read-only config dependency in DESIGN §9
and §10.

**Why:** `reload from disk` and the generic configuration promise should give
the board and commands the same workbench. Startup-only configuration would
require a restart rule the author never chose. Retaining the established root
keeps a reload from unexpectedly switching repositories.

## Execution rules

Every unit:

- first shows the defect with a test that fails, where a test can show it,
  then fixes it and keeps the test;
- makes the smallest change that fixes the defect, in the code the unit names;
- implements cited recommended decisions as written and never makes another
  design decision itself; when the fix turns out to need one the plan does not
  make, it finishes BLOCKED and names the decision;
- changes DESIGN in the same commit when changing its format, commands, keys,
  lint rules or package boundaries;
- ends with `scripts/verify` and `go test ./...` green. Stage new files before
  Nix checks. A unit changing go.mod or go.sum updates vendorHash by AGENTS's
  procedure and runs `nix build .#benchpress`. The final unit also runs
  `nix build .#benchpress`, `nix flake check`, `benchpress lint` and
  `nix run . -- ls`.

Coverage-only gaps describe currently correct production behavior. Add the
missing assertions first, verify that the specific regression described in the
unit makes its focused test fail, restore the production implementation, and
keep the stronger test. Do not change correct behavior merely to get an initial
failure. Documentation corrections use the named textual checks instead.

## Stage 1 — Preserve the formats

### 1.1 Reject configuration keys with the wrong case

**Goal:** only exact `dir`, `kinds` and `areas` keys are accepted. Check TOML
metadata keys explicitly instead of relying solely on Undecoded; preserve
defaults and valid explicit empty lists.

**Evidence:** gap, 1.1 F1: `internal/config/config.go:40,50` — the decoder
matches fields case-insensitively, but IsDefined is case-sensitive. `Kinds`
passes the unknown-key check and its value is overwritten with defaults.

**Start from:** Load in `internal/config/config.go` and its tests. Inspect
metadata key components, rejecting unknown or nested names with the offending
key in the error; do not add a TOML dependency.

**Done when:** `TestLoadRejectsCaseVariantKeys` rejects `Kinds`, `Dir`, `AREAS`
and an unrelated key, names each rejected key, and accepts the three lowercase
keys. The execution checks pass.

### 1.2 Walk past a directory named like the configuration file

**Goal:** FindRoot recognizes a regular configuration file, following a symlink
to such a file, and continues upward past a `.benchpress.toml` directory.

**Evidence:** gap, 1.1 F2: `internal/config/config.go:63` — any successful Stat
ends discovery, so a directory shadows a genuine file above it.

**Start from:** FindRoot in `internal/config/config.go`; require the returned
file information to describe a regular file before choosing the root.

**Done when:** `TestFindRootSkipsConfigDirectory` finds a real configuration
above the impostor directory and returns ErrNoRoot when none exists above it.
The execution checks pass.

### 1.3 Preserve CRLF when appending a field and reject empty titles

**Goal:** fix the two local topic slips without changing unrelated bytes.
An appended field before an unterminated closing delimiter inherits an existing
preceding terminator, using LF only if no terminator exists. A bare `#` or
whitespace-only heading text produces a title issue.

**Evidence:** slips, 1.2 F1 and F3: `internal/topic/topic.go:179-181,317-321` —
Set defaults to LF when the closing delimiter has no terminator; heading1
explicitly accepts an empty heading as a title.

**Start from:** Set, terminator, heading1 and Parse in `internal/topic/topic.go`.
Keep empty headings recognizable for the one-H1 check, but never accept one as
a valid title without a title issue.

**Done when:** `TestSetAppendCRLFWithoutFinalNewline` covers both populated and
empty front matter with byte-exact expectations; `TestParseEmptyTitle` checks
`#` and `#   ` at their actual line. Existing multiple-heading checks still
pass, as do the execution checks.

### 1.4 Exclude image tokens from collected links

**Goal:** implement D1 in `internal/topic/topic.go` and document it in DESIGN
§3. Exclude an entire image token rather than accidentally collecting a link
from its alt text; preserve neighboring ordinary links.

**Evidence:** design, 1.2 F2: `internal/topic/topic.go:367-385` — the collector
walks over `!` and parses the following bracket as an ordinary link. Parse
uses this on body lines outside fences, and lint also calls LineLinks on every
Next item, so both entry points are affected.

**Start from:** collectLinks, parseLink and LineLinks. Distinguish an image's
unescaped `!` from a literal escaped exclamation mark before an ordinary link.

**Done when:** `TestImagesAreNotLinks` covers Parse and LineLinks with
`![alt](missing.png)`, a Markdown target, and a real adjacent link; images
contribute no links and the ordinary link remains. The execution checks pass.

### 1.5 Separate a newly added Next block from unterminated prose

**Goal:** Render adds a separator before the first added item when the no-item
source's last head line lacks a newline, using the source's newline style.
Leave all existing head bytes intact and preserve the final-newline convention.

**Evidence:** slip, 1.3 F1: `internal/next/next.go:123-129` — head lines are
joined directly to the new item, producing `Some prose.1. [a](a.md)`.

**Start from:** Next.Render in `internal/next/next.go`; keep the no-operation
round trip unchanged, including a completely empty source.

**Done when:** `TestAddAfterUnterminatedProse` covers Add and AddAt at 1 with
LF and CRLF sources, requires the separating newline, and checks the new final
item's bytes. The execution checks pass.

### 1.6 Preserve each parsed Next item's line terminator

**Goal:** a canonical numbered Next block with mixed LF and CRLF terminators
round-trips unchanged. Retain terminator metadata on parsed items and carry it
through moves; new items use the file's detected style. An absent terminator
must be supplied when that item is no longer the final output line. Continue
normalizing item markers and gathering prose as DESIGN §4 requires.

**Evidence:** gap, 1.3 F2: `internal/next/next.go:49,64,126-127,234` — Render
uses the first detected newline for every item, losing later terminators.

**Start from:** Item, Parse and Render in `internal/next/next.go`. Preserve the
existing Add, Remove, Move and missing-final-newline behavior.

**Done when:** `TestRoundTripMixedLineEndings` covers both mixed orders and a
missing final newline; `TestMoveMixedLineEndings` checks that no moved or added
item welds onto the next line. Existing prose-gathering tests and execution
checks pass.

## Stage 2 — Locate plan files accurately

### 2.1 Read literal paths and full branch refs from git

**Goal:** find non-ASCII branch-only plan names and report actual branch names
even when tags have colliding names. Use full `refs/heads/...` refs internally,
strip that prefix only for displayed results, and use NUL-delimited ls-tree
path output. Update the documented command forms in DESIGN §5 in this commit.

**Evidence:** slips, 1.4 F1 and F4: `internal/repo/repo.go:99,109,116` —
newline-split ls-tree output retains C quoting; `refname:short` disambiguates
collisions as `heads/foo` rather than returning the branch's name.

**Start from:** branches, branchHolds and Find in `internal/repo/repo.go`.
Keep one for-each-ref and one ls-tree per branch over the whole path set.

**Done when:** `TestFindNonASCIIPlanOnBranch` returns `other` for
`docs/plans/café.md`; `TestFindBranchNameWithTagCollision` returns `foo` for
branch foo alongside tag foo and branch heads/foo. The command-count test and
execution checks pass.

### 2.2 Surface repository failures without breaking operation outside git

**Goal:** implement D2 in `internal/repo/repo.go` and DESIGN §5. Return the
actual useful diagnostic rather than a false absent-plan map on git failures.

**Evidence:** design, 1.4 F2: `internal/repo/repo.go:63-75,98-103` — Find
returns early without git for an empty path set, but for every nonempty set
it suppresses all branch-enumeration errors. Corrupt packed-refs is thereby
indistinguishable from being outside a repository.

**Start from:** New's runner, branches and Find. Preserve error wrapping and
the command families, and classify only the specified C-locale diagnostic as
non-repository. Do not treat all exit-128 errors as that case.

**Done when:** `TestFindCorruptRefsReturnsError` uses a real temporary repo
and malformed packed-refs and requires its diagnostic; `TestFindOutsideRepository`
still returns worktree hits without error. `TestFindDoesNotSuppressOtherFailures`
covers cancellation and an unavailable git executable. The execution checks pass.

### 2.3 Assert that every holding branch is returned

**Goal:** pin the existing all-branches result independently of command counts.

**Evidence:** gap, 1.4 F3: `internal/repo/repo_test.go:117-152` — its fixture
has a.md on main and other, but discards Find's result; the other result tests
assert at most one branch.

**Start from:** TestFindRunsOneLsTreePerBranch in `internal/repo/repo_test.go`,
or a separate test sharing its real-repository setup. Keep HOME temporary and
the command-count assertions intact.

**Done when:** `TestFindEveryHoldingBranch` asserts exactly `worktree`, `main`,
`other` for a.md and detects a temporary regression that stops after the first
holding branch. The execution checks pass.

### 2.4 Refuse directories as found plan files

**Goal:** worktree plan lookup recognizes regular files rather than any path
that Stat accepts. Following a symlink to a regular file remains valid. A
directory alone must not satisfy a planned topic's plan reference.

**Evidence:** gap, 1.12 F1: `internal/repo/repo.go:92-94` accepts directories;
`internal/lint/lint.go:129-130` trusts a nonempty location result. The board's
settled-state prompt prefills `docs/plans/`, so accepting it unchanged reaches
this path through both SetState and the CLI.

**Start from:** inWorktree in `internal/repo/repo.go`; require regular-file
information. Recursive ls-tree already returns leaf paths rather than a
directory itself; retain that behavior and test it with and without a slash.

**Done when:** `TestFindDirectoryIsNotPlan` returns no locations for
`docs/plans` and `docs/plans/`, inside and outside git, but finds an actual plan
file. Command integration `TestLintDirectoryPlanGone` requires a plan-gone
error and exit 1 for a directory-only plan. The execution checks pass.

## Stage 3 — Align listing and lint contracts

### 3.1 Match Next targets to the actual topic path

**Goal:** both board listing/index lookup and lint accept a Next target only
when its normalized relative path names the loaded, directly contained topic
file. Require `.md`; accept equivalent paths such as `./a.md`, but reject
`a`, `sub/a.md` and `../outside/a.md` beside topic a.md. Invalid items neither
occupy a topic's Next position nor make a later valid item a duplicate.

**Evidence:** slip, 1.6 F1, folding in 1.5 F1:
`internal/lint/lint.go:140,191-195` strips directories, while
`internal/board/board.go:285-289` also accepts missing suffixes. Listing,
NextIndex and the board's add/remove/move decisions share the board helper.

**Start from:** both targetSlug helpers, Board.Listing and nextIndex in
`internal/board/edit.go`. Use path normalization relative to the workbench;
do not introduce a new package or change the topic-file layout rule.

**Done when:** `TestNextTargetsMustNameTopicFiles` in board and lint covers
all examples, present and absent outside files, and an invalid item followed
by `[A](a.md)` without next-duplicate. `TestLSInvalidNextTarget` checks text
and JSON keep a in its area group, and board index checks do not match invalid
targets. The execution checks pass.

### 3.2 Classify missing Markdown links correctly with dir set to dot

**Goal:** a missing `missing.md` under `dir = "."` is a topic-link error,
while a path escaping that directory remains subject to the outside-link rule.

**Evidence:** slip, 1.6 F2: `internal/lint/lint.go:166-167` compares a cleaned
path with the prefix `./`, which cleaned in-directory paths never carry.

**Start from:** checkLink in `internal/lint/lint.go`. Use a normalized
containment check that handles dot and `..` without a string-prefix accident.

**Done when:** `TestTopicLinkWithRootDirectory` checks the rule, error severity
and source line; command `TestLintRootDirectoryLinkExitsOne` requires exit 1.
A paired outside link still produces the existing warning. The execution
checks pass.

### 3.3 Supply lint's existence query and test its rules entirely in memory

**Goal:** implement D3 across board, lint, all Check callers and DESIGN §10.
Remove filesystem access from the lint implementation and lint package tests.
Keep command and TUI integration tests on temporary directories. Document the
existing TUI locator dependency as part of this package-boundary correction.

**Evidence:** design, 1.6 F5: `internal/lint/lint.go:185-189` stats the board
root and `internal/lint/lint_test.go:36-60` writes every fixture to disk.
Also a documentation slip, 1.10 F6 with duplicate 1.11 F1:
`internal/tui/model.go:17,32,64` already uses repo.Locator, while DESIGN §10
lists only board and lint. Reload asks that locator only for nonempty Plans
with a nonnil locator.

**Start from:** Check and checker.exists; add the root-relative Board method,
then update `cmd/benchpress/lint.go`, `cmd/benchpress/board.go` and
`internal/tui/actions.go` together. Build lint test boards using topic.Parse
and next.Parse and supply an in-memory path set, including NEXT.md presence.

**Done when:** `TestCheckUsesSuppliedExistence` distinguishes present and
missing links without disk; all rule tests still pass, and
`rg '"os"|"path/filepath"|t.TempDir|board.Load' internal/lint` returns no
matches. DESIGN §10 records both injected facts and the retained repo locator
dependency. The execution checks pass.

### 3.4 Assert the severity of every lint rule

**Goal:** pin every error and warning row of DESIGN §6, including structural
issues forwarded from topic.Parse.

**Evidence:** gap, 1.6 F3: `internal/lint/lint_test.go:70-78` checks only the
rule name. Most production Error-to-Warning changes therefore keep tests green.

**Start from:** requireOnly and all rule tests in `internal/lint/lint_test.go`.
Take an explicit expected severity and assert it at every call; retain minimal
fixtures that produce only the intended rule.

**Done when:** `go test ./internal/lint` pins all §6 rows, including area;
temporarily downgrading plan-gone makes TestPlanGone fail, and temporarily
upgrading link makes TestLink fail. The execution checks pass after restoration.

### 3.5 Exercise lint command ordering within one path

**Goal:** command text and JSON tests detect missing or reversed line ordering,
as well as preserve their path-order and exit-code coverage.

**Evidence:** gap, 1.6 F4: `cmd/benchpress/lint_test.go:36-44,58-79,92-118`
has no path with findings on different lines.

**Start from:** lintFixture and both command output tests. Include two issues
on distinct lines of one file whose collection order differs from line order
(for example a later structural issue collected before an earlier value issue), so
deleting the sort comparison also fails; two links collected in source order
alone would not prove that.

**Done when:** `TestLintTextOrdersAndExitsOne` and TestLintJSON assert the
exact path/line order and fail with the line comparator deleted or reversed.
The execution checks pass after restoration.

### 3.6 Pin an empty JSON problem array on a clean workbench

**Goal:** clean `lint --json` returns an empty array, not null, and succeeds.

**Evidence:** gap, 1.6 F6: `cmd/benchpress/lint.go:57-63` normalizes nil, but
the sole JSON test at `cmd/benchpress/lint_test.go:92` has findings.

**Start from:** add a clean command fixture and test in
`cmd/benchpress/lint_test.go`, with an existing NEXT.md and no planned topics.
Check the JSON value's shape, not merely a decoded slice's length.

**Done when:** `TestLintCleanJSON` requires exit 0 and `"problems": []`, and
fails when the nil conversion is temporarily removed. The execution checks pass.

### 3.7 Pin configured area order before unconfigured areas

**Goal:** listing tests distinguish configured order from alphabetic order and
require unconfigured areas after configured ones, alphabetically among themselves.

**Evidence:** gap, 1.5 F2: `internal/board/board_test.go:12-18,60-77` uses
alphabetical configuration and no mixed configured/unconfigured fixture.

**Start from:** board listing fixtures and TestListingOrder. Use an explicit
configuration `maps, economy` and topics in both plus unconfigured `ant, zebra`;
loading must still list topics with lint-invalid configured values.

**Done when:** `TestListingConfiguredAreaOrder` asserts maps, economy, ant,
zebra and fails if areaKey ignores Config.Areas. The execution checks pass.

### 3.8 Pin the order of multiple Next entries

**Goal:** preserve authored Next order in both returned nextSlugs and leading
entries, and in command text and JSON output.

**Evidence:** gap, 1.5 F3: `internal/board/board_test.go:67` and
`cmd/benchpress/ls_test.go:29` contain only one Next item.

**Start from:** listing and ls fixtures. Put second.md before first.md in Next
with titles whose alphabetical order is the reverse, then assert both arrays
and rendered leading order rather than mere substring presence.

**Done when:** `TestListingNextOrder` and `TestLSNextOrder` fail if Next items
are sorted or iterated in reverse, for text and JSON. The execution checks pass.

### 3.9 Exercise every listing filter through the command

**Goal:** board tests exclude an unwanted kind, and command tests prove each
state, kind and area flag is wired to the matching filter, including repetition.

**Evidence:** gap, 1.5 F4: `internal/board/board_test.go:132-138` includes
every available kind; `cmd/benchpress/ls_test.go:60-135` uses no filter flags.

**Start from:** TestListingFilter and ls command tests. Add an excluded kind
and fixtures whose states, kinds and areas select different topic sets, checking
Next membership as well as topics. Repeated values are OR within that flag;
different flags intersect, matching the existing Filter.match implementation.

**Done when:** `TestListingFilter` detects an ignored Kinds filter and
`TestLSFilters` checks each flag in text and JSON and detects swapped Kinds/Areas
wiring. The execution checks pass.

### 3.10 Pin planned topics after settled topics outside Next

**Goal:** verify open, settled, planned state ordering inside a single area
without Next ordering masking it.

**Evidence:** gap, 1.5 F5: `internal/board/board_test.go:60-77` puts its only
planned topic in Next, bypassing stateRank's planned branch.

**Start from:** add a focused board ordering fixture with no Next entries and
one topic of each state in the same area; choose titles in a conflicting order.

**Done when:** `TestListingGroupedStateOrder` asserts open, settled, planned
and fails if planned is ranked before settled.
The execution checks pass.

### 3.11 Assert the first parse issue in broken-topic command text

**Goal:** guard the requirement that ls names a broken topic and its first
parse issue instead of silently reducing it to a slug.

**Evidence:** gap, 1.5 F6: `cmd/benchpress/ls.go:101-104` prints the message,
but `cmd/benchpress/ls_test.go:66` asks only for `bad`.

**Start from:** TestLS and its bad.md fixture in `cmd/benchpress/ls_test.go`.
Assert that the bad topic's line includes `file does not start with ---`.

**Done when:** TestLS fails when entryLine's broken branch temporarily returns
only the slug; restore it and pass the execution checks.

## Stage 4 — Make writes and their coverage reliable

### 4.1 Read escaped closing brackets in link labels

**Goal:** implement the reader half of D4 without changing target parsing or
adding a Markdown dependency. Both topic links and leading Next links locate
the closing label bracket after skipping backslash-escaped characters.

**Evidence:** design, reader half of 1.7 F1:
`internal/topic/topic.go:416-424` and `internal/next/next.go:164-175` use the
first closing bracket, so a generated title with a bracket can be broken or
misread as a link to the inner target.

**Start from:** topic.parseLink and next.leadingTarget, with byte-level parser
tests. Add the supported escape syntax to DESIGN §3 and §4 in this commit.
Keep the parsers' existing code-span, image and relative-target policies.

**Done when:** `TestLinkEscapedLabels` in topic and next reads escaped
brackets, doubled backslashes and an escaped inline-link-looking label as the
outer target. Plain-link tests and execution checks pass.

### 4.2 Escape titles when writing Next links

**Goal:** implement D4's producer half in board.nextText, retaining the topic
title while writing a label both readers understand. A title that looks like
an inline link must never redirect the generated Next entry to another topic.

**Evidence:** design, producer half of 1.7 F1:
`internal/board/edit.go:235-239`, reached by New only with toNext, and by
NextAdd for an existing topic — raw title concatenation closes the outer
label early. A wrong inner target can survive lint and defeat removal.

**Start from:** nextText and tests for New/NextAdd. Escape `\`, `[` and `]`
in the correct order; use the readers supporting D4. Do not escape or rewrite
the heading stored in the topic file.

**Done when:** `TestNextTitleEscaping` and command `TestNewNextBracketTitle`
cover `Fix [thing]`, `A [b](c.md)` with c.md present, and backslashes. Reparse
NEXT.md, require the intended target, lint without new Next errors, and require
next rm and rm to remove the correct entry. The execution checks pass.

### 4.3 Reject negative Next insertion positions before mutation

**Goal:** keep 0 as append and reject negative positions with an out-of-range
error, leaving disk and the parsed Next unchanged.

**Evidence:** slip, 1.7 F2: `internal/board/edit.go:180-188` sends every
nonpositive at value to Add; `cmd/benchpress/next.go` explicitly defaults 0
to append but exposes negative integer arguments too.

**Start from:** addNextItem and NextAdd's comment in `internal/board/edit.go`;
validate before constructing a changed Next. Keep positive AddAt behavior.

**Done when:** `TestNextAddFailures` includes -1 and snapshots NEXT.md and
Next.Items; command `TestNextAddNegativePosition` errors without changes.
Existing append and insertion tests and execution checks pass.

### 4.4 Roll back multi-file operations when their second step fails

**Goal:** complete board's existing failed-operation guarantee for New with
toNext and Remove of a topic in Next. Prepare required data before mutation;
on a failed second step, restore the prior topic's presence, bytes and mode,
NEXT.md's prior presence/bytes/mode, and the in-memory Next. Do not merely swap
write order and leave the opposite half-change. Keep temporary files inside
the workbench, clean them up, and preserve individual atomic file replacements.

**Evidence:** gap, 1.7 F3: `internal/board/edit.go:43-49,138-150` creates or
deletes the topic before writing Next, then returns the second error without
rollback. Remove takes the two-file branch only when nextIndex finds the topic.

**Start from:** New, Remove, addNextItem and writeNext in
`internal/board/edit.go`. Use a narrow internal failure seam if needed for
deterministic tests; do not create a general transaction framework. Retain
rollback errors alongside the original error rather than silently losing them.
No crash-recovery or cross-process transaction promise is introduced.

**Done when:** `TestNewNextWriteFailureRollsBack` and
`TestRemoveNextWriteFailureRollsBack` inject the second-step failure and assert
the complete before/after snapshots, including a missing original NEXT.md and
a nondefault topic mode. Retrying on the same Board succeeds. The execution
checks pass.

### 4.5 Pin permission preservation on topic and Next rewrites

**Goal:** test the already implemented preservation of regular permission bits
without changing the writer contract.

**Evidence:** gap, reclassifying 1.7 F4:
`internal/board/edit.go:252-273` preserves destination permissions, but
`internal/board/edit_test.go` never checks them.

**Start from:** board operation tests under t.TempDir. Chmod an existing topic
and NEXT.md to 0600; exercise SetState and a Next rewrite, then stat each file.

**Done when:** `TestWritesPreserveMode` requires 0600 for both paths and fails
when writeAtomic's destination-mode preservation is temporarily removed. The
execution checks pass after restoration.

### 4.6 Remove partial init files and replace skills atomically

**Goal:** a failed createOnly write or close removes its newly created partial
target so a retry can install complete bytes; a failed `init --skill` rewrite
leaves the previous skill intact. Keep create-only O_EXCL behavior for config,
Next and an unforced skill install, and use a same-directory temporary file and
rename for forced replacement, preserving an existing skill's permissions.

**Evidence:** gap, 1.8 F1: `cmd/benchpress/init.go:147-165` leaves the target
after a failed write or close; `:92-100` truncates an existing skill directly.
Plain init then skips that existing partial skill indefinitely.

**Start from:** createOnly and the rewriteSkill branch in
`cmd/benchpress/init.go`. Use a small local helper and a deterministic write
failure seam, or a subprocess-limited write test; do not limit the test process
globally and do not weaken no-overwrite behavior.

**Done when:** `TestCreateOnlyWriteFailureLeavesNoTarget` and
`TestInitSkillRewriteFailurePreservesOldFile` fail partway through writing,
assert absence or original bytes respectively, and verify a retry installs all
of benchpress.Skill. Close-failure cleanup and no-overwrite tests also pass,
along with the execution checks.

### 4.7 Report errors inspecting Claude's skill directory

**Goal:** only a not-exist result skips the optional Claude skill directory;
other stat errors propagate instead of falsely reporting complete success.

**Evidence:** slip, 1.8 F2: `cmd/benchpress/init.go:107-114` silently ignores
every failed Stat. existingOrDefaultConfig already distinguishes not-exist
from other errors.

**Start from:** installWorkbench's claudeSkills check. Retain the existing
behavior for an absent directory and for a successfully statted non-directory.

**Done when:** `TestInitClaudeSkillsStatError` uses a symlink loop, or a
non-searchable directory when not root, and requires a stat error. Normal absent
and directory cases still pass, as do the execution checks.

### 4.8 Verify repeat init leaves an existing Claude symlink untouched

**Goal:** include the symlink in the second-init no-change guarantee, checking
that it is not replaced as well as keeping its target string.

**Evidence:** gap, reclassifying 1.8 F3:
`cmd/benchpress/init_test.go:47-63` never creates .claude/skills; initSnapshot
at `:129-138` records a target but cannot detect recreation of the same link.

**Start from:** TestInitSecondRunChangesNothing. Create .claude/skills before
the first init, snapshot its installed link with Lstat, and compare file
identity/metadata with the second run in addition to the existing byte snapshot.

**Done when:** the strengthened test fails for a temporary unlink/recreate
regression even when the recreated target is identical. A correct second init
prints nothing and leaves all snapshots unchanged. The execution checks pass.

### 4.9 Pin init's discovered root and configured directory

**Goal:** tests protect init from creating a nested workbench inside an
initialized root, and from ignoring an existing nondefault dir.

**Evidence:** gap, reclassifying 1.8 F4: `cmd/benchpress/init.go:53-65,72-86`
implements both behaviors; init tests use fresh roots and the custom-config
test at `cmd/benchpress/init_test.go:71` never checks the Next location.

**Start from:** init command tests. Initialize a root then run from a child;
separately supply `dir = "custom"` before init with no default workbench.

**Done when:** `TestInitFromChildUsesRoot` requires no nested config, Next or
skill; `TestInitHonorsConfiguredDirectory` requires custom/NEXT.md, unchanged
config bytes and no default-dir Next. These detect the respective always-cwd
and always-default-dir regressions. The execution checks pass.

## Stage 5 — Keep the board truthful and usable

### 5.1 Detect an actual stdout terminal

**Goal:** no-argument benchpress runs ls when stdout is /dev/null, a pipe or a
regular file; only an actual terminal opens the interactive board.

**Evidence:** slip, reclassifying 1.9 F1:
`cmd/benchpress/board.go:15-22` confuses any character device with a terminal;
`cmd/benchpress/main.go:43-48` uses that predicate to choose the interface.

**Start from:** stdoutIsTerminal. Use the existing charmbracelet/x/term
IsTerminal implementation (already a transitive dependency), making it direct
if Go module tooling requires it; avoid adding a new library family. Update
vendorHash in the same commit if go.mod or go.sum changes.

**Done when:** `TestStdoutDevNullIsNotTerminal` safely saves/restores stdout,
and the existing redirected no-argument listing test passes. The execution
checks, including the required package build on any module change, pass.

### 5.2 Honor modal quit keys and delete a whole filter rune

**Goal:** Ctrl+C quits before any modal key dispatch. q quits from help and
delete confirmation, but remains text in the filter and title/kind/area/plan
inputs. Backspace removes the final UTF-8 rune from a filter, preserving valid
text. Keep ?/Escape help closing and Escape input cancellation.

**Evidence:** slips, 1.9 F2 with duplicate 1.10 F2, and 1.9 F4:
`internal/tui/model.go:168-188` dispatches help, prompts, confirmation and
filter before quitting; `:252-254` slices one byte off the filter. promptKey
forwards keys to textinput and confirmKey treats any non-y key as refusal.

**Start from:** Model.key and filterKey in `internal/tui/model.go`; inspect
promptKey and confirmKey in actions.go but centralize the quit dispatch.

**Done when:** `TestQuitFromEveryMode` sends KeyCtrlC in normal, help, filter,
each input prompt and delete confirmation and executes the returned command
to require tea.QuitMsg, without writes. q has the stated text/nontext behavior.
`TestFilterBackspaceRune` types é, deletes it and requires an empty valid-UTF-8
filter. The execution checks pass.

### 5.3 Render a named, nonselectable Next heading

**Goal:** a nonempty visible Next section has a Next heading before its
numbered entries; it is not a selectable topic. Numbering begins at 1 and
filtering out all Next entries removes the otherwise empty heading.

**Evidence:** gap, 1.9 F3: `internal/tui/model.go:313-317` appends only
rowNext items; `internal/tui/view.go:64-70` has no Next heading to render.

**Start from:** rebuild and row rendering, using the existing nonselectable
group-header handling where suitable. Do not derive item numbers from the
total row count after adding a header. Adjust group-label assertions so they
still test the non-Next groups explicitly.

**Done when:** `TestViewNamesNextSection` asserts heading-before-first-item,
numbers 1 and 2, and an initial selection on the first topic; navigation skips
the header and a filter with no matching Next topic hides it. The execution
checks pass.

### 5.4 Test preview bodies, selection changes and scrolling

**Goal:** guard the actual rendered preview, rather than finding a title that
also exists in the list. Test switching selected bodies and preview scrolling.

**Evidence:** gap, 1.9 F5: `internal/tui/model_test.go:200-209` checks only
titles, leaving `internal/tui/model.go:585` and View's right pane unguarded.

**Start from:** model tests with two topics containing distinct short body
phrases that appear nowhere in their titles; one body has enough paragraphs
to scroll. Send window-size and key messages, never drive a terminal.

**Done when:** `TestPreviewShowsSelectedBody` finds the selected unique phrase
in the wide View, replaces it on selection, and detects removing SetContent or
the preview pane. `TestPreviewScrollKeys` proves Ctrl+D increases YOffset and
Ctrl+U decreases it. The execution checks pass.

### 5.5 Test filtering by a slug independent of its title

**Goal:** pin slug matching with assertions on the actual filtered entries.

**Evidence:** gap, reclassifying 1.9 F6:
`internal/tui/model_test.go:138-147` uses drafts, also in its title, and asks
only whether any selection remains; `internal/tui/model.go:349-355` checks
slug along with title, kind and area.

**Start from:** TestFilterMatchesSlugKindAndArea; use raised-ground, which
does not occur literally in its spaced title, and assert the exact selected
slug and absence of unmatched topic rows. Assert the exact matching sets for
the kind and area cases too; their current existence-only assertions are weak
for the same reason.

**Done when:** the test fails with e.Slug temporarily removed, and also fails
if filtering is bypassed entirely. The execution checks pass after restoration.

### 5.6 Publish loaded board data when plan enrichment fails

**Goal:** implement D5 in the reload, change and editor-exit paths and DESIGN
§9. Publishing new file data and marking derived information unavailable must
not depend on plan lookup succeeding.

**Evidence:** design, reclassifying 1.10 F1:
`internal/tui/actions.go:76-95` loads files but returns before assignment on
findPlans failure; change at `:59-67` then displays only the error. This path
requires a nonnil locator and at least one nonempty planned path. Current
for-each-ref failures are swallowed upstream until Stage 2; an ls-tree failure
or a failing injected locator already reaches it today.

**Start from:** reload, change, findPlans, editorClosedMsg handling, and
lintSummary/status rendering. Clear stale locations/problems/error marks and
represent unavailable lint separately from a clean empty problem list.

**Done when:** `TestSavedChangeSurvivesLocatorFailure` starts with an open
selected topic and another planned path; after s, disk, entry and preview agree
on settled, the status says the change was saved and reload failed, and the
next s opens the plan prompt. `TestReloadFailureInvalidatesDerivedData` rejects
stale plan locations and a false clean summary and checks later recovery.
The execution checks pass.

### 5.7 Reload configuration along with the workbench

**Goal:** implement D6 in reload and DESIGN §9/§10, preserving D5's handling
of successfully saved changes. Keep the same root and preserve selection by
slug when the newly configured directory still contains it.

**Evidence:** design, reclassifying 1.12 F2:
`internal/tui/actions.go:76` reuses m.board.Config, while command loadBoard
at `cmd/benchpress/ls.go:62-66` loads config each invocation. r, successful
operations and editor completion all call the same reload method.

**Start from:** reload in `internal/tui/actions.go`; call config.Load before
board.Load. Update TUI test fixtures that currently construct Config without
writing .benchpress.toml; do not silently default missing files in production.

**Done when:** `TestReloadReadsConfiguration` changes kinds and areas and
successfully creates a topic using the new values after r;
`TestReloadSwitchesConfiguredDirectory` sees the new workbench and subsequent
writes go there. `TestReloadInvalidConfiguration` preserves coherent prior
data with an error, including a saved-change case whose disk state is still
published using the old config. The execution checks pass.

### 5.8 Assert that a completed change refreshes lint state

**Goal:** protect post-change re-linting, its summary and its topic error marks.

**Evidence:** gap, 1.10 F3: `internal/tui/actions.go:84,90-91` does the work,
but no test in actions_test inspects the resulting lint state. The existing
worktreeLocator reports every plan found and masks plan-gone changes.

**Start from:** action model tests with a fake locator that returns no
locations. Plan the selected settled topic to a missing path, completing the
normal s prompt, then clear its plan by cycling back to open.

**Done when:** `TestChangeRerunsLint` requires plan-gone, its path error mark
and the error summary after planning, and their removal after returning open.
It fails if lint.Check or the problems/error-path assignments are removed from
reload. The execution checks pass after restoration.

### 5.9 Assert editor exit updates the model's loaded body

**Goal:** make the editor-reload test depend on the model receiving disk edits.

**Evidence:** gap, reclassifying 1.10 F4:
`internal/tui/actions_test.go:255-283` inspects the edited file and empty
status, both unchanged by removing model.go's editorClosedMsg reload call.

**Start from:** TestEditorKeyReloads; keep its synchronous execProcess seam
and appending script, then assert the model's bodies and preview picked up the
new phrase after processing the returned editorClosedMsg.

**Done when:** the strengthened test fails if editorClosedMsg no longer reloads
and passes with the existing behavior. The execution checks pass.

### 5.10 Exercise Enter's wide-layout editor branch

**Goal:** Enter opens the selected topic's editor at 100 columns or wider,
while the existing below-100 preview toggle remains covered.

**Evidence:** gap, 1.10 F5: `internal/tui/model.go:232-238` has separate
narrow and wide branches; the editor test sends only e.

**Start from:** parameterize the synchronous editor test for e and KeyEnter,
or add a separate action test, setting width exactly 100 or larger.

**Done when:** `TestEnterOpensEditorWhenWide` proves the script ran and the
model reloaded its edit, and fails if the wide Enter branch is deleted.
TestNarrowHidesPreviewAndEnterToggles and execution checks pass.

### 5.11 Pin VISUAL before EDITOR

**Goal:** the editor command honors VISUAL when both environment variables
are set, preserving EDITOR and vi fallbacks.

**Evidence:** gap, 1.10 F7: `internal/tui/actions.go:44-54` prefers VISUAL,
but the only editor test at `internal/tui/actions_test.go:265` clears it.

**Start from:** editor action tests using the existing synchronous seam. Set
VISUAL and EDITOR to different scripts with distinguishable effects; do not
depend on an installed interactive editor.

**Done when:** `TestEditorPrefersVisual` proves only the VISUAL script ran and
fails for reversed precedence or ignored VISUAL. The EDITOR test still passes,
and `TestEditorDefaultCommand` checks vi's argv without launching it. The
execution checks pass.

### 5.12 State Enter's narrow-layout exception in help

**Goal:** the full key list accurately describes e and Enter at all widths.

**Evidence:** slip, 1.12 F3: `internal/tui/view.go:178` says both open the
editor, while `internal/tui/model.go:232-238` toggles preview below 100
columns, as DESIGN §9 already requires.

**Start from:** helpView in `internal/tui/view.go`. Include the below-100
exception in plain text; no key-binding or design change is needed.

**Done when:** `TestHelpExplainsNarrowEnter` opens help on a narrow sized
model and requires the editor binding and preview-toggle exception. The
execution checks pass.

## Stage 6 — Correct the remaining descriptions

### 6.1 Describe board's file-writing role in its package comment

**Goal:** remove the false claim that board never writes and describe loading,
ordering and the existing §7 file operations accurately.

**Evidence:** slip, 1.7 F5: `internal/board/board.go:1-2` says nothing writes,
while `internal/board/edit.go` implements the writes and DESIGN §10 assigns
them to board.

**Start from:** only the package comment in `internal/board/board.go`; retain
the small, factual comment style. No new test or API change is needed.

**Done when:** `rg 'nothing writes' internal/board` has no matches and the
package comment agrees with DESIGN §10. The execution checks pass.

### 6.2 Describe the deferred-view premise with the topic-file exceptions

**Goal:** qualify the deferred topic's Now sentence to describe directly
contained topic Markdown files, excluding README.md and NEXT.md. Preserve its
deferred questions and the decision to postpone fuller cross-branch views.

**Evidence:** slip, 1.11 F3:
`docs/workbench/show-topics-another-branch-has-deleted-or-changed.md:13` says
every Markdown file, but `internal/board/board.go:86-93` ignores subdirectories,
README.md and NEXT.md as DESIGN §2 requires.

**Start from:** only that topic's Now wording. This is the final fix unit;
run the full final verification specified in the execution rules, including
the package, sandboxed checks and dogfood commands.

**Done when:** the Now paragraph says topic files directly in dir and explicitly
names the excluded README.md and NEXT.md; a textual comparison with DESIGN §2
agrees, and all final verification commands pass.
