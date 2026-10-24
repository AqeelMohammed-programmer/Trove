# Open questions

Unresolved questions that need the user's answer. Each entry states the
question, why it matters, the recommended answer and what is blocked.

## Open

### 1. License

**Question:** which license should the project use?

**Status:** deferred by the user on 2026-10-24. No `LICENSE` file exists and
no license is declared in the module.

**Why it matters:** without a license nobody may legally reuse the code, and
`AGENTS.md` section 9.3 requires the file before a release.

**Recommendation:** MIT for a small personal Go project, unless copyleft is
wanted.

**Blocked:** nothing during development; only a public release.

### 2. Non-HTTP URL schemes

**Question:** should `mailto:`, `ftp:` or other absolute URL schemes be allowed
for link attachments, or only `http`/`https`?

**Status:** the spec allows only `http` and `https` (`A-2`) and lists this as
open decision 8.1.1. Until answered, the implementation follows the spec and
rejects everything else.

**Recommendation:** keep `http` and `https` only. Other schemes add handling
(downloads, mail clients) that the spec does not describe.

**Blocked:** nothing; a decision is needed before M3 (attachments) is finished
if the answer is to widen it, because `A-2` would then need a spec change.

## Resolved

| Date | Question | Answer |
|---|---|---|
| 2026-10-24 | Product name | **Trove**, taken from the repository folder name. Recorded in the spec header and in `internal/app/name.go`. |
| 2026-10-24 | Where the specification lives | Moved to `docs/spec.md` and marked **Accepted**. |
