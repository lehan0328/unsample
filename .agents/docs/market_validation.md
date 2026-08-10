# Market Validation: Evidence vs. Assumptions

**Goal:** Separate hard evidence from analysis for each top idea. You just learned with Touchstone that analysis ≠ reality. Let's not repeat that.

---

## The Honest Framework

| Level | What It Means | Example |
|---|---|---|
| 🟢 **Hard Evidence** | Observable behavior — people spending money or time | Companies publicly writing about their pain with names attached |
| 🟡 **Soft Evidence** | Community signals — complaints, questions, upvotes | Reddit threads, SO questions, GitHub issues |
| 🔴 **Assumption** | Analyst logic — "they should want this" | "Every company with an OSS core needs this" |

---

## Idea #1: Copybara SaaS (Cross-Repo Sync)

### 🟢 Hard Evidence (things that actually happened)

| Evidence | Source | What It Proves |
|---|---|---|
| **Dagster** publicly blogged about building bidirectional Copybara sync | dagster.io blog | At least 1 company invested significant engineering in this exact problem |
| **Octopus Deploy** moved C# client lib to monorepo, uses Copybara for public mirror | octopus.com blog | 2nd named company with this problem |
| `google/copybara` has ~2K stars, 200+ issues | GitHub | Thousands of developers are aware of the tool; hundreds have hit problems |
| Community-maintained GitHub Actions (`olivr/copybara-action`, `will-molloy/copybara-action`) exist | GitHub | Pain was bad enough that people built wrapper automations |
| `git subtree` has well-documented recurring breakage with "Squash and Merge" | Stack Overflow | The DIY alternative is provably fragile |

### 🟡 Soft Evidence (complaints but no money spent)

| Evidence | Source |
|---|---|
| Reddit threads describe "dependency hell" and "nightmare" coordinating multi-repo changes | r/devops, r/git, r/programming |
| Stack Overflow has recurring questions about "keep two repos in sync" | stackoverflow.com |
| Copybara's own docs acknowledge bidirectional sync is "notoriously difficult" | Google's web search summary |
| `git subtree` users describe "merge nightmares" and recommend "fresh start workaround" (delete + re-add) | Stack Overflow |

### 🔴 Assumptions (unvalidated)

| Claim | Status |
|---|---|
| "5-10 hours/week maintaining sync scripts" | ❌ No source. I made this up based on general estimates. |
| "$250/month per repo pair WTP" | ❌ No source. I extrapolated from engineer salary math. |
| "Customers self-identify via copybara.sky files" | ⚠️ Partially true — public repos are searchable, but most enterprise use is private |
| "Zero SaaS competitors" | ⚠️ True as of research, but AI coding agents (Augment, Cursor) are starting to handle multi-repo context differently |
| "Narrow TAM is fine for bootstrapping" | ⚠️ Logical but untested — how narrow is too narrow? |

### Honest Assessment

**The problem is real.** Named companies (Dagster, Octopus Deploy) have publicly documented their pain. The DIY tools (`git subtree`, raw Copybara) are demonstrably brittle.

**But the market is genuinely small.** Only companies with a specific architecture pattern need this: private monorepo + public OSS mirror, OR multi-repo with shared code. That's maybe a few hundred companies, not thousands.

**The biggest risk isn't competition — it's TAM.** You could build a perfect product and max out at 30 customers.

---

## Idea #2: Break-Glass CLI (JIT Prod Access)

### Evidence Summary

| Type | Evidence |
|---|---|
| 🟢 Hard | Apono acquired by 1Password for $250-300M (June 2026) — validates the market exists at scale |
| 🟢 Hard | Hoop.dev (YC-backed, MIT-licensed) exists — someone already built this, and got funded |
| 🟢 Hard | $150M+ in VC funding across 6+ startups in JIT/PAM |
| 🔴 Assumption | "A simpler CLI-only version would differentiate" — but Hoop.dev is already open-source |

**Verdict:** The market is massive and proven, but competition is fierce. This is a VC-scale opportunity, not a bootstrapper one.

---

## Idea #3: Example-Based PR Review Rules

### Evidence Summary

| Type | Evidence |
|---|---|
| 🟡 Soft | Developers constantly share "code review nits" on Twitter/Reddit — qualitative pain |
| 🟡 Soft | Semgrep has 11K+ stars — adjacent tool validation |
| 🔴 Assumption | "Paste before/after examples → get a CI rule" is novel UX — untested with users |
| 🔴 Assumption | Teams would pay for this vs. just writing Semgrep rules |

**Verdict:** Novel UX angle but zero hard evidence of WTP. Would need to demo the concept to validate.

---

## Idea #4: Tombstone (Dead Code Elimination)

### Evidence Summary

| Type | Evidence |
|---|---|
| 🟢 Hard | Sentry launched Reaper (July 2025) — they're building this with massive distribution |
| 🟢 Hard | Azul Intelligence Cloud already ships it for Java |
| 🟢 Hard | CodeAnt AI (YC W24, $2M seed) — funded competitor |
| 🔴 Assumption | "Polyglot web stacks are still open" — window is closing |

**Verdict:** Market validated by VC money, but you'd be entering against Sentry. Not a bootstrapper fight.

---

## Idea #5: Reviewer Routing & PR Context

### Evidence Summary

| Type | Evidence |
|---|---|
| 🟡 Soft | PullApprove, Reviewpad exist — some demand |
| 🔴 Assumption | "Load balancing + PTO awareness is the novel piece" — untested |
| 🔴 Assumption | Git blame gets 80% — the remaining 20% may not justify a product |

**Verdict:** Weak evidence. Partial solutions exist and seem "good enough."

---

## So What Now?

Here's the uncomfortable truth from the evidence:

| Idea | Problem Real? | Market Big Enough? | Can You Win? |
|---|---|---|---|
| Copybara SaaS | ✅ Yes (named companies) | ⚠️ Maybe too small | ✅ No competition |
| Break-Glass CLI | ✅ Yes ($250M acquisition) | ✅ Huge | ❌ Hoop.dev, Apono, 6+ funded startups |
| PR Review Rules | ⚠️ Unclear | ⚠️ Unclear | ⚠️ Semgrep looms |
| Tombstone | ✅ Yes (Sentry building it) | ✅ Large | ❌ Sentry has distribution |
| Touchstone | ✅ Yes (ADA lawsuits) | ✅ Large | ⚠️ "Vitamin" problem — 1 star proved it |

### The Core Tension

The ideas where the problem is clearly real AND the market is big enough have funded competition. The ideas where you can win have small or uncertain markets.

### What I'd Actually Do (if I were being honest, not analytical)

**Before building ANYTHING else, validate demand in 48 hours:**

1. **For Copybara SaaS** — search GitHub for repos with `copy.bara.sky`, find the maintainers' emails or Twitter handles, DM 10 of them: "We're building a hosted Copybara that handles bidirectional sync. Would you switch from your current setup?" If 3+ say yes → build it. If 0 → don't.

2. **For any idea** — the Touchstone lesson is: **don't build for 3 days then post on Reddit.** Instead: describe the product in 3 sentences, post to the community BEFORE writing code, and see if people ask "where do I sign up?" vs. "cool, good luck."

3. **Consider ideas outside this list.** All 5 were generated from "Google internal tools" framing. Maybe the strongest bootstrapper opportunity isn't in dev tools at all. What problems do YOU face daily that you'd pay to solve?

---

## Concrete 48-Hour Validation Steps

### Option A: Validate Copybara SaaS (highest evidence)
- [ ] Search GitHub for `filename:copy.bara.sky`, collect 20 repo owners
- [ ] Draft a 3-sentence cold DM: "I saw you use Copybara for [X]. We're building a hosted version that handles bidirectional sync with zero config. Would this save you time?"
- [ ] Send to 20 people (Twitter DM, GitHub issue, email)
- [ ] **Gate:** If ≥3 respond with interest → build an MVP landing page → collect emails → then build
- [ ] **Gate:** If 0-1 respond → kill the idea

### Option B: Start Fresh
- [ ] List 5 problems YOU personally face (not theoretical market gaps)
- [ ] For each: Can you describe it in one sentence? Would YOU pay $50/mo to solve it?
- [ ] Pick the one where the answer is most obvious
- [ ] Post to relevant community: "I'm building X. Would you use it?"
- [ ] Only start coding after ≥5 "yes, when?" responses
