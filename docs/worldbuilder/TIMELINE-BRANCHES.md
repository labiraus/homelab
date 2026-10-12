# Reviewed snapshot branches

Timeline branches create a fixed copy of timeline-specific canon through the existing proposal, review and publication flow. The user confirmed fixed parent snapshots with reviewed updates on 2026-10-12. Adopting later parent changes requires explicit reviewed proposals against branch records; there is no automatic inheritance. Existing timeline labels and attributes never imply a parent.

## What is copied

The parent is an explicit timeline at an immutable canon revision in the same authorized world. The server copies every record scoped to that timeline, including facts, relationships, events, scoped rules and any scoped catalog records. It preserves all fictional intervals, including exact signed-int64 endpoints; current search, pagination and fictional-time filters do not narrow the copy. It creates one new timeline descriptor, assigns new stable record IDs and remaps typed references between copied records. Free-form prose is retained without guessing which text represents an ID.

World-wide records remain shared: unscoped entity/type/source catalog, world settings, calendars, review policy and unscoped rules continue to use the destination's current canon. This is a frozen timeline assertion set, not a frozen copy of the entire world. A later reviewed shared-schema or global-rule change still affects validation in every timeline. Historical-parent copies are revalidated against the current shared catalog. Missing, hidden or incompatible references are refused, never replaced by an unrelated source. Scoped timeline descriptors are refused rather than silently omitted or interpreted as nested inheritance.

Shared rule reference closures must remain visible too. For example, a world-wide rule attached to a parent-scoped type cannot silently stop applying when that type is copied under a new identity: the branch is refused until the rule/type scope is explicitly resolved through canon review.

The complete branch must fit the ordinary 100-operation proposal limit: one new timeline and at most 99 copied records. The existing 5,000-record world limit also applies. Larger branches fail in full; there is no truncation or multi-publication approximation. An empty parent can create an empty alternate timeline. New branch and copied identities must never have been published in that world, including subsequently deleted identities.

## API and governance

`POST /api/worlds/{world}/timeline-branches` accepts:

```json
{
  "id": "11111111-1111-4111-8111-111111111111",
  "baseRevision": 5,
  "parentRevision": 3,
  "parentTimeline": "main",
  "timeline": "alternate",
  "label": "Alternate history",
  "reason": "Explore a different outcome"
}
```

Both revisions are required nonnegative integers; explicit revision 0 is valid. `baseRevision` must be current. Only an editor, admin or owner can create the draft, and the usual archive, membership and same-origin rules apply. The endpoint returns an ordinary `Change` with status `draft`. No canon, revision or outbox event changes until normal submission, approval and atomic publication succeed. Current quorum and the existing explicit superuser self-review exception apply unchanged.

The proposal and immutable `timeline.branch` audit are written in the same transaction. Proposal UUIDs are normalized before auditing, so equivalent UUID spelling preserves origin and retry identity. Exact actor/request retries return the original proposal even after later publication; changing other request content under the same UUID returns 409. A failed audit write rolls back the draft as well. Stale publication is refused. A stale branch must be prepared again against inspected current canon with a fresh proposal UUID and fresh reviews: ordinary record-by-record rebase is refused so it cannot omit some copies or lose the branch's creation origin, including when the source UUID in the URL uses upper-case hex.

`GET /api/worlds/{world}/timelines/{timeline}/origin?revision=N` verifies membership and the timeline's presence in the requested snapshot. It returns the original request, branch publication revision and source-ID to copied-ID mapping from the published proposal/audit. An ordinary isolated timeline returns `null`; an absent historical timeline returns 404. These are creation details, not a claim that current records still equal their original copies. Editable labels/attributes cannot fabricate this response. No new database schema or migration is required; existing schema-8 snapshots, proposals, identity history and immutable audit store the complete state.

Subsequent branch edits are ordinary proposals against the copied IDs. Parent edits and deletions do not update the copies. Citations resolve the copied canonical ID at the branch's queried revision/timeline; the origin endpoint separately resolves creation ancestry to the immutable parent snapshot. Re-import/merge automation and automatic parent synchronization remain outside this bounded contract.

## Browser and acceptance

**Prepare timeline branch** pins the displayed timeline/revision and the observed current revision before form editing. Changing wiki filters does not repin the draft. The form explains the shared catalog and complete-copy limit. An uncertain submission retains its UUID and full request; an unchanged retry remains possible after canon advances, without rebasing. Creating a proposal directs the user to the ordinary review queue. **View timeline origin** reads only the displayed scope and is available to readers.

Unit tests cover complete copy, cross-record remapping, full-range time, unchanged source maps, other-timeline exclusion, exact record-count boundaries, hidden/missing references and collision refusal. Disposable PostgreSQL cases exercise permissions, required pins, exact retries, publication/history/citations, stale rebase refusal, retired identities and injected audit rollback. A third Chromium flow creates a branch while the wiki is filtered, reviews/publishes it, updates its parent and verifies unchanged branch evidence plus historical origin. Run it through the [existing offline browser/database harness](OFFLINE-ACCEPTANCE.md). Current execution and publication evidence are recorded in [implementation status](IMPLEMENTATION-STATUS.md).
