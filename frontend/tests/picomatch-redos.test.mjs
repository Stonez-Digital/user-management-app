import assert from "node:assert/strict";
import { performance } from "node:perf_hooks";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);
const picomatch = require("picomatch");

test("single-branch leading quantifiers are literalized before regex compilation", () => {
  const expected = new Map([
    ["+(*a)", String.raw`^(?:\\+\\(\\*a\\))$`],
    ["*(*a)", String.raw`^(?:\\*\\(\\*a\\))$`],
    ["+(a|*a)", String.raw`^(?:\\+\\(a\\|\\*a\\))$`],
  ]);

  for (const [pattern, source] of expected) {
    assert.equal(picomatch.makeRe(pattern).source, source, pattern);

    const candidate = "a".repeat(30) + "!";
    const started = performance.now();
    assert.equal(picomatch.isMatch(candidate, pattern), false, pattern);
    assert.ok(
      performance.now() - started < 250,
      `pattern ${pattern} exceeded the ReDoS regression budget`,
    );
  }
});

test("non-risky repeated extglobs preserve existing matching behavior", () => {
  assert.equal(picomatch.isMatch("abc", "+(*)"), true);
  assert.equal(picomatch.isMatch("abc", "+(a*)"), true);
  assert.equal(picomatch.isMatch("aaa", "+(a*)"), true);
  assert.equal(picomatch.isMatch("abab", "+(a|b)"), true);
  assert.equal(picomatch.isMatch("aaaa", "+(*(a))"), true);
});
