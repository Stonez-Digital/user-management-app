import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import { apiRequest, ApiError } from "../frontend/lib/api-client.mjs";

const originalFetch = globalThis.fetch;
const originalStorage = globalThis.localStorage;

afterEach(() => {
  globalThis.fetch = originalFetch;
  if (originalStorage === undefined) delete globalThis.localStorage;
  else globalThis.localStorage = originalStorage;
});

test("adds the current bearer token and parses a successful JSON response", async () => {
  globalThis.localStorage = { getItem: (key) => key === "access_token" ? "test-token" : null };
  globalThis.fetch = async (url, options) => {
    assert.equal(url, "/backend/admin/academic-sessions");
    assert.equal(options.headers.get("Authorization"), "Bearer test-token");
    assert.equal(options.headers.get("Content-Type"), "application/json");
    return new Response(JSON.stringify({ sessions: [] }), { status: 200 });
  };

  assert.deepEqual(await apiRequest("/admin/academic-sessions"), { sessions: [] });
});

test("preserves backend validation codes and formats field details", async () => {
  globalThis.localStorage = { getItem: () => "test-token" };
  globalThis.fetch = async () => new Response(JSON.stringify({
    error: {
      code: "validation_error",
      message: "request validation failed",
      details: [{ field: "teacher_id", rule: "required" }, { field: "term_id", rule: "required" }],
    },
  }), { status: 400 });

  await assert.rejects(
    apiRequest("/admin/teacher-assignments", { method: "POST", body: "{}" }),
    (error) => error instanceof ApiError &&
      error.status === 400 &&
      error.code === "validation_error" &&
      error.message.includes("teacher_id: required") &&
      error.message.includes("term_id: required") &&
      Array.isArray(error.details),
  );
});

test("handles non-JSON gateway errors without throwing a JSON parse error", async () => {
  globalThis.localStorage = { getItem: () => null };
  globalThis.fetch = async () => new Response("upstream unavailable", { status: 502 });

  await assert.rejects(
    apiRequest("/admin/academic-sessions"),
    (error) => error instanceof ApiError &&
      error.status === 502 &&
      error.message === "upstream unavailable",
  );
});

test("normalizes unauthorized responses to the shared session-expired error", async () => {
  globalThis.localStorage = { getItem: () => "expired-token" };
  globalThis.fetch = async () => new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 });

  await assert.rejects(
    apiRequest("/admin/academic-sessions"),
    (error) => error instanceof ApiError && error.code === "session_expired" && error.message === "Session expired",
  );
});
