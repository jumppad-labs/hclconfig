---
tags: [hcl, parsing, testing, fixtures, diagnostics]
---

# HCL halts on a malformed block header, so two bad headers yield one diagnostic

The HCL parser stops at a malformed **block header** rather than recovering and
continuing. A fixture containing two blocks with bad headers therefore produces
only **one** diagnostic, not two — the second fault is never reached.

This is a trap when writing a fixture meant to prove that *every* problem in a
file is reported. The obvious fixture — two deliberately broken blocks — will
appear to prove the opposite of what it was written for, because the parser
never sees the second one.

**What to do instead:** build such a fixture from *recoverable* faults, which the
parser does continue past. Two `resource` blocks each missing their name label
work, because they are well-formed enough to parse and are caught later by the
block loop rather than by the parser itself.

Measured diagnostic counts, for calibration:

| fault | diagnostics |
|---|---|
| two malformed block headers | **1** |
| one unterminated quoted string | **3** |
| one unclosed `${` | **2** |
| a block missing its name label | 0 parse diagnostics (caught by the block loop) |

Note the asymmetry the table shows: a *single* malformation often yields several
related diagnostics, while two *separate* header malformations yield one. Counts
are not a proxy for how many things are wrong with a file, so a test asserting
"all problems reported" should assert on the **content** of what was reported,
not on a bare count.
