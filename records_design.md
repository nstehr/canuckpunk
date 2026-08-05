# Records & Institutional Memory — Design

## Thesis

In-game documents are stored in git; their revision history is part of the fiction.
Institutions hold competing versions of the same events, and that disagreement is a
permanent, first-class feature rather than a conflict to be resolved. SQLite continues
to own live gameplay state.

---

## The three objects

Everything below hangs off this distinction. Keep it legible to players — these must
look nothing alike in the TUI, because their contracts are completely different.

| Object | Store | Mutable | Visible to others | In-character |
|---|---|---|---|---|
| **Private notes** | SQLite | Yes | Never | No — the *player's*, not the character's |
| **Journal** | Git | Yes, visibly | Under discovery | Yes |
| **Institutional records** | Git | Via amendment | Per institution | Yes |

**Private notes** are an out-of-character thinking surface. Always private, freely
editable, freely deletable, no revision IDs, no merge, no discovery. There is no code
path that exposes them — "usually private" would decay, and if notes can ever surface
players self-censor and the feature loses its purpose.

> "Private" means private from other players. The operator can read the SQLite file.
> Don't let the in-game copy promise more than that.

**Journal** is the in-character notebook. Git-backed, discoverable, and revisable *with
consequences* — see below.

**Institutional records** are canon: dossiers, case files, reports, personnel files.

---

## Scope

**The test:** *are this document's revisions part of the fiction?*

| Git owns | SQLite owns |
|---|---|
| Dossiers, case files, reports, personnel files | Characters, locations, inventory |
| Journals | Private notes |
| Institutional canon | Sessions, presence, auth (SSH pubkey → username) |
| Discovery productions | Activity logs, scheduled work, timers, deadlines |
| | Derived indexes over git (rebuildable) |
| | Revision-ID allocation counters |

**Deliberately excluded from git:**

- **Activity logs.** Each git write is several fsyncs — roughly 100× a SQLite INSERT.
  Correct for records, wrong for a log.
- **Authored content.** The markdown in `narratives/` keeps its own repo and pipeline.
  Separate system, separate lifecycle.

**Characters have both** — a live state row *and* an official personnel file. This is
correct, not duplication. Named here so it doesn't later read as drift.

**Core invariant:** *git is the source of truth; SQLite is a derived index.* No
transaction spans both, so the index must be rebuildable by walking history. Write that
path first, not last.

---

## Document model

Records are **structured** — named sections and fields, not freeform prose. This is the
load-bearing decision.

Merges operate per-section. A conflict is therefore *"you both amended **Disposition**"*
— a two-option choice — rather than *"lines 12–19 disagree,"* which has no friendly
presentation. Freeform text lives inside a section; sections are the merge unit and are
never merged across.

---

## Identity

**Players see institutional revision IDs.** `RCMP-1974-00412/r3`. Never raw SHAs.

A bureaucracy issues file numbers, not hex digests — the institutional ID is more
on-theme than a hash, not less. It's also readable, typeable, referenceable in dialogue,
and it decouples display identity from storage so the backend can change without
breaking in-fiction references.

**SHAs remain the underlying immutable identity** and stay internal. The dedupe property
still pays off: the system notices two records share a blob hash and surfaces it *as
fiction* — "this report is textually identical to one filed in Sudbury in '71." The
player never compares hashes; the hash does the work invisibly.

**Allocation rules:**

- Allocated at commit time from a transactional SQLite counter.
- **Monotonic, never reused** — including for revisions on dispute branches later
  overruled. "r4 was filed and overruled" is a legitimate record; two competing r4s is a
  bug.
- Scoped per issuing institution.

---

## Authority

**There is no `main`.** Each institution holds its own canon:

```
refs/heads/<institution>/canon
```

Conflicting institutional memories are permanent and legitimate — the RCMP's version of
an event and the Ministry's version may diverge forever. For bureaucratic alt-history
that isn't a defect, it's the thesis.

This separates two things that are easily conflated:

### Intra-institutional conflict → dispute (should resolve)

```
refs/disputes/<institution>/<record>/<player>
```

- The write always lands somewhere durable and attributed. Never blocked, never lost.
- **The docket is a ref scan.** No status table, no reconciler. This is the property
  SQLite can't replicate cleanly, and the main reason git earns its place here.
- **Adjudication is a merge commit** — records who ruled, when, and what both sides said.
  The ruling's provenance is itself content.
- **Rejection is also a merge** (ours-strategy). Never delete a dispute ref: unreachable
  objects get gc'd and you lose the history you bought git for. "Filed and overruled"
  beats "never happened."
- **Re-merge at adjudication time.** Canon moves while a branch sits, so adjudication is
  a fresh three-way, not a replay of the original.
- **A second conflicting edit joins the existing dispute** as another commit. Always
  branch off canon, never off an open dispute — otherwise adjudication goes N-way and you
  grow dispute trees. Models co-signers on a filing.
- **Disputes time out.** Players log off forever. On lapse: canon wins, dispute archived,
  ruling recorded as *unadjudicated, lapsed* — and the lapse is a visible in-game event.

### Inter-institutional divergence → permanent (never resolves)

Needs no timeout. Merging one body's canon into another's is a *political act* —
gameplay, not cleanup.

Diffing two institutions' canon for the same record is a discovery mechanic: the files
disagree, and learning that is content.

Access control falls out naturally — a player sees the canon of the institution they're
acting within.

---

## Concurrency

Wiki-style optimistic, per institution:

1. Player opens a record → base blob hash recorded.
2. On save, if that institution's canon has moved → three-way merge against the base.
3. **Clean → commit silently.** The majority case and the real win: disjoint section
   edits merge invisibly instead of clobbering. Players never learn git is involved.
4. **Conflict → dispute branch** (above).

Where the fiction allows, **default resolution is "keep both"** — contested variants,
both attributed, both visible. A file with contradictory annexes is thematically correct
and can't lose a player's text. Force a single canonical value only where fiction demands
it, and adjudicate in-fiction.

Optional mechanic: "draft privately, then *file* it" maps to a real branch — and *"the
registry rejected your filing as contested"* is a better error than any UI we'd design.

---

## Journal

The in-character notebook. `refs/heads/person/<character>/journal`.

The character is their own micro-institution, so this reuses the authority model with no
new concepts. Single author means no concurrent edits, so the dispute and adjudication
machinery simply goes unused here.

**Revision history is the story element.** Institutional amendment is routine and boring;
a *person* quietly revising what they wrote is damning. "You edited this entry on the
14th" is an accusation.

**In-fiction date is separate from commit time.** The character sets the entry's date —
forgeable, it's what the page says. The commit timestamp is system truth, hidden under
normal reading. An entry dated the 3rd but committed on the 14th is *backdating*, and the
discrepancy surfaces only under scrutiny — a document examiner, a rival institution's
analysis. In-fiction forgery with real detectability, where detection is a capability
someone must possess rather than something the UI just announces.

**Destruction behaves like destroying a notebook.** Drop the ref, mark it destroyed
in-fiction; the objects remain reachable through any production that already happened.
Destroying your journal removes *future* access but cannot unmake a copy already produced
under discovery. "You burned it, but we have the July production" falls out of the storage
model with no special-casing.

> **Caution.** The journal deliberately reintroduces the deletion problem for
> player-authored freeform text. Acceptable *only* if the contract is legible at the point
> of writing — it must look and feel like a permanent in-character document, visibly
> distinct from private notes, so writing there is informed consent. If the two ever look
> similar in the TUI, players will put out-of-character thoughts in the permanent one and
> be justifiably upset.

---

## Discovery

Not seizure. Seizure is a raid; discovery is a *procedure*, and a procedure has surface
area a single instant doesn't. Commissions of inquiry and ATIP-style access requests are
about as Canadian-institutional as it gets — a request that comes back with black bars is
the whole aesthetic in one object.

**Subject:** institutional records and journals. Never private notes.

**Why it works:**

- The subject knows and participates. You see what was asked for and choose what to
  produce — asymmetric information as the core of the interaction.
- Multi-beat: request → scope → compliance → production. Each is a place to put gameplay.
- **Non-compliance is itself a filing.** "Failed to produce" enters the record
  permanently, so refusing has a cost that isn't just a referee saying no.

**Redaction is where content-addressed history earns its keep.** If unredacted text later
enters the record by another route — another institution's copy, a leak, a second
production — you can diff them and the redaction becomes *provable*.

**It feeds the authority model.** Production lands the document in the *requesting*
institution's canon. The RCMP's file now contains a produced copy; the Ministry's doesn't.
Divergence created by procedure rather than accident.

**To design:**

- **Scope disputes are intra-institutional disputes.** What falls within a request is
  contestable — existing branch/adjudication machinery, no new concepts.
- **Requests need deadlines**, same shape as dispute lapse, or a player stalls
  indefinitely. Lapse into "failed to produce" and make it a visible event.
- The *request* is transient state with scheduled work → SQLite. The *production* is
  fiction-historical → git. The scope rule decides it cleanly.

---

## Provenance — content only

Documents cite one another, and those citations are **markdown content, not a modelled
system**. No reference graph, no index, no traversal layer.

```
Case 41-B
  ↓ references
Survey Revision
  ↓ cites
Church Register
  ↓ copied into
Registry Filing
```

Deliberate: if following a citation is one keypress, it's a wiki; if it's a prose
reference the player has to chase through the archive — needing access, maybe a discovery
request — it's detective work. The friction *is* the gameplay.

Citations still ride inside versioned blobs, so provenance stays historically accurate: a
citation added in r3 is visibly added in r3. Fiction without machinery.

**Write citations to a consistent format** — `SUR-18-44/r2`. Convention only, no parser.
It preserves the option of retroactively extracting the graph later (stale-citation
detection, quote verification) without rewriting history. Inconsistent formatting is the
only part of this that's expensive to undo.

Longer term the archive is arguably about *evidence* rather than documents — documents as
containers that cite, quote, transmit, and reinterpret. Noted, not built.

---

## Implementation

**Shell out to the git CLI.** Not a compromise at this write volume.

`Makefile:109-123` cross-compiles with `CGO_ENABLED=0` and `scp`s static binaries to the
droplet; the project is on `modernc.org/sqlite` specifically to stay pure-Go. cgo isn't
one new dependency — it takes out that deploy model, requiring either a cross toolchain or
build-on-droplet with `libgit2-dev`, plus runtime version matching or a static link.

**Primitives:**

- **`git merge-tree --write-tree`** (2.38+) — three-way merge against bare repos with **no
  worktree and no index**. Returns the merged tree OID plus machine-parseable conflict
  output naming conflicted paths. This is the structured result the per-section UI needs,
  and worktree-free means no index-lock contention.
- `hash-object`, `commit-tree`, `cat-file` — object plumbing.
- `update-ref` — compare-and-swap, so ref writes are lock-free.

Add `git` to `provision.sh`.

**Build behind a thin interface** — `hashObject`, `mergeTree`, `updateRef`, `readBlob` —
so swapping to libgit2/git2go later is a leaf change, not a redesign. Revisit only at high
write volume; check git2go's maintenance and version-pinning story at that point.

**Repo shape:** bare repo, refs as above. Loose refs get slow in the thousands — pack them.

---

## Open questions

**1. Shared refs or linked documents?** When two institutions hold a version of the same
event:

- *One document, N institutional refs* — cheap diffing, meaningful merge, shared identity.
- *N documents with a "concerns the same event" link* — more bureaucratically realistic
  (each body opens its own file with its own numbering), but no cheap diff or merge.

Likely provenance-based: shared refs where a document was genuinely transmitted or copied
between bodies, separate documents where each institution authored independently. That
distinction is probably itself a mechanic — a leaked file and parallel investigations are
different stories.

**2. Deletion.** Scoping git to fiction-historical documents bounds the problem, and
*"the record does not forget"* is defensible in-fiction — but fiction is not a defense for
genuinely abusive user content. Private notes being SQLite removes the most sensitive
category. The journal deliberately puts some back. Needs a real answer before real users.

---

## Build order

1. Object layer + SQLite index + **rebuild-from-history path**.
2. Revision-ID allocation.
3. Single-institution canon, linear commits. No merging yet.
4. Three-way merge on save — silent clean-merge only. **This is where most of the value
   is.**
5. Dispute branches with "keep both" as the *only* resolution.
6. **Instrument:** log conflict frequency by record type and section.
7. Journal.
8. Multi-institution canon and divergence.
9. Discovery.
10. Adjudication UI — only for the record types step 6 proves need it.

Steps 4 and 6 are the ones not to skip. The silent merge is the invisible win, and the
instrumentation tells you whether the adjudication UI is worth building at all.
