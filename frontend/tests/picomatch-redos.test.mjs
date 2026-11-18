import assert from "node:assert/strict";
import { performance } from "node:perf_hooks";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);
const picomatch = require("picomatch");

test("single-branch repeated extglobs cannot trigger catastrophic backtracking", () => {
  for (const pattern of ["+(*)", "+(*a)", "*(*)", "*(*a)", "+(?*a)"]) {
    const candidate = "a".repeat(48) + "!";
    const started = performance.now();

    assert.equal(picomatch.isMatch(candidate, pattern), false, pattern);

    const elapsed = performance.now() - started;
    assert.ok(
      elapsed < 250,
      `pattern ${pattern} took ${elapsed.toFixed(1)}ms`,
    );
  }
});

test("safe repeated extglob character branches retain their matching behavior", () => {
  assert.equal(picomatch.isMatch("abab", "+(a|b)"), true);
  assert.equal(picomatch.isMatch("aaaa", "+(*(a))"), true);
  assert.equal(picomatch.isMatch("cccc", "+(*(c))"), true);
});
