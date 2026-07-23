# Questions for the design agent (claude.ai/design)

Relay these to the Grove TUI design conversation. Each is self-contained (the design
agent has no grove-code context) and carries a recommended answer, per the handoff brief.

Everything else in the implementation plan is locked to its recommended answer and needs
no design input. This is the **only** item held open pending design.

---

## Q · The upgrade key `u` vs. `U` adjacency (relates to D9 and R8)

**Context.** In grove's current TUI, two keys on the same physical key do very different things:

- `u` (lowercase) — open the upgrade modal (D9).
- `U` (uppercase) — switch to the selected worktree **and force-start its containers** (a heavier, side-effectful action).

Because they share a key, a fat-fingered `U` when reaching for `u` (or vice-versa) can trigger a container force-start unexpectedly. R8 already flagged "relocate `u` upgrade" as an open item, and the layered-command model (D11) gives us a natural home for it.

**Proposed solution (recommended).** Move the upgrade action off the top-level `u`. Two viable homes — both keep the header `⭡x.y.z` badge as the primary affordance:

- **(a)** Put it in the `g` (goto) layer as **`g u`** — "goto upgrade," discoverable via the which-key strip. *(Recommended — keeps a keyboard path, removes the hazard, fits the layered model.)*
- **(b)** Surface it only through the header badge + a `?`-help entry, with no dedicated hot key.

`U` (switch + force-start) stays as-is.

**Decision needed.** Agree with relocating upgrade off `u`? If so, (a) `g u` or (b) badge/help only? Or keep `u` top-level and accept the adjacency? This changes a documented key, so we're confirming before locking it into the plan (currently tracked as open item Q5 in §6 of the implementation plan).
