import assert from "node:assert/strict";
import test from "node:test";
import { schoolPathForHostname, schoolSlugFromHostname } from "../frontend/lib/school-domain.js";

test("resolves a single school subdomain", () => {
  assert.equal(schoolSlugFromHostname("bedrock.stonezdigital.com.ng"), "bedrock");
});

test("normalizes host casing, trailing dot, and port", () => {
  assert.equal(schoolSlugFromHostname("Bedrock.stonezdigital.com.ng:443"), "bedrock");
  assert.equal(schoolSlugFromHostname("bedrock.stonezdigital.com.ng."), "bedrock");
});

test("does not treat the organisation portfolio or reserved hosts as schools", () => {
  for (const host of ["stonezdigital.com.ng", "www.stonezdigital.com.ng", "app.stonezdigital.com.ng", "api.stonezdigital.com.ng", "mail.stonezdigital.com.ng"]) {
    assert.equal(schoolSlugFromHostname(host), null, host);
  }
});

test("rejects nested, malformed, and unrelated hostnames", () => {
  assert.equal(schoolSlugFromHostname("a.bedrock.stonezdigital.com.ng"), null);
  assert.equal(schoolSlugFromHostname("-bedrock.stonezdigital.com.ng"), null);
  assert.equal(schoolSlugFromHostname("bedrock.example.com"), null);
});

test("maps a school subdomain root to its existing slug route", () => {
  assert.equal(schoolPathForHostname("bedrock.stonezdigital.com.ng", "/", ""), "/school/bedrock");
});

test("preserves school subpaths and query strings", () => {
  assert.equal(schoolPathForHostname("bedrock.stonezdigital.com.ng", "/login", "?next=%2Fdashboard"), "/school/bedrock/login?next=%2Fdashboard");
});

test("pins explicit school routes to the hostname tenant", () => {
  assert.equal(schoolPathForHostname("bedrock.stonezdigital.com.ng", "/school/other/blog"), "/school/bedrock/blog");
});

test("does not rewrite a route already scoped to the same school", () => {
  assert.equal(schoolPathForHostname("bedrock.stonezdigital.com.ng", "/school/bedrock/login"), null);
});
