# ADR-001: Apache License 2.0 for Dingo

**Date:** 2026-03-28
**Status:** Accepted
**Author:** MadAppGang

---

## Context

Dingo is a company-backed, open-source meta-language for Go (module: `github.com/MadAppGang/dingo`). The project includes:

- A CLI transpiler (`dingo build`, `dingo run`, `dingo go`)
- An LSP server for IDE integration
- Editor plugins for VS Code, Neovim, and GoLand
- A runtime library (`dgo`) that user programs import directly

The `dgo` runtime creates a specific licensing constraint: any license that imposes obligations on _downstream_ code would require users to open-source their programs. That rules out copyleft licenses immediately.

Within the permissive tier (MIT, BSD-3-Clause, Apache 2.0), the key differentiator is patent coverage. Dingo is a compiler-class tool. Transpiler and code transformation techniques can be the subject of patents, and MadAppGang does not have a separate corporate patent pledge infrastructure.

---

## Decision

Dingo uses **Apache License 2.0**.

---

## Consequences

### Positive

**Patent grant (Section 3).** Apache 2.0 requires each contributor to grant users a royalty-free patent license covering their contributions. MIT and BSD-3-Clause contain no patent language. If MadAppGang or any contributor holds patents on transpiler or source-mapping techniques, users are automatically protected — without any separate pledge required.

**Patent retaliation clause.** If a user sues any other Dingo user for patent infringement related to Dingo, their Apache 2.0 license terminates. This creates a defensive shield across the entire user base.

**Runtime safety.** Apache 2.0 imposes no obligations on programs that import `dgo`. User code remains under whatever license the user chooses. This matches MIT and BSD-3-Clause behavior for library consumers.

**Enterprise adoption.** Large organizations run license scans before approving internal tooling. Apache 2.0 appears on virtually every enterprise-approved list. MIT often passes too, but Apache 2.0's explicit patent grant removes the one ambiguity that causes legal teams to stall. Google, Microsoft, JetBrains, and the Apache Foundation all ship under Apache 2.0 for this reason.

**Precedent from analogous tools.** The closest analogs to Dingo — TypeScript (Microsoft), Kotlin (JetBrains), and swc — all chose Apache 2.0. These are company-backed language tools that target existing ecosystems. Community-driven tools like Babel use MIT, but that reflects a different origin and risk profile.

**Practical equivalence to Go's BSD-3-Clause.** Go uses BSD-3-Clause backed by Google's separate patent pledge. MadAppGang has no equivalent infrastructure, so Apache 2.0's built-in patent clause provides the same effective protection without needing a separate legal document.

**Competitive differentiation.** Borgo, a comparable Go meta-language, ships with no license. That makes it legally unusable for any serious project — contributors cannot grant rights they haven't specified. A clear, permissive license is itself a trust signal.

### Negative

**Attribution requirement.** Apache 2.0 requires preserving copyright notices and the license text in distributions. MIT is slightly simpler in this regard. In practice, this rarely causes friction for a CLI tool or library.

**File size.** The Apache 2.0 license text is longer than MIT. Minor point, but worth noting for completeness.

---

## Alternatives considered

### MIT

Simpler text, no attribution complexity. No patent grant. For a hobby project or community tool without a corporate backer, MIT is appropriate. For a company-backed transpiler where contributors may hold patents on the underlying techniques, the absence of a patent grant is a meaningful gap. Rejected.

### BSD-3-Clause

Matches Go's own license, which creates a sense of ecosystem alignment. Still no patent grant. Would require MadAppGang to publish a separate patent pledge to achieve equivalent protection — additional legal overhead for no practical benefit over Apache 2.0. Rejected.

### MIT + Apache 2.0 dual-license (Rust's approach)

Maximizes flexibility for users who need one license or the other. Adds complexity to every contribution and legal review. Could be reconsidered later if the community requests it. Not adopted at this stage.

### MPL-2.0

File-level copyleft: modifications to Dingo's files must be released, but user code is unaffected. Uncommon in the Go ecosystem. Adds friction without a clear benefit over Apache 2.0. Rejected.

### GPL / AGPL / LGPL

All forms of copyleft impose conditions on downstream code that links against `dgo`. Incompatible with the requirement that user programs remain unlicensed. Rejected.

---

**Reviewed by:** MadAppGang engineering
