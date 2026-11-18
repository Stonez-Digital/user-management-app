import assert from "node:assert/strict";
import { installAuthFetch } from "../frontend/lib/auth-fetch.mjs";

const values = new Map([
  ["access_token", "old-access"],
  ["refresh_token", "old-refresh"],
]);
globalThis.localStorage = {
  getItem: (key) => values.get(key) ?? null,
  setItem: (key, value) => values.set(key, String(value)),
  removeItem: (key) => values.delete(key),
};

let lockTail = Promise.resolve();
Object.defineProperty(globalThis, "navigator", { configurable: true, value: {
  locks: {
    request(_name, _options, callback) {
      const run = lockTail.then(callback);
      lockTail = run.then(() => undefined, () => undefined);
      return run;
    },
  },
} });

const expiredEvents = [];
globalThis.window = globalThis;
globalThis.location = {
  origin: "https://school.example",
  pathname: "/dashboard",
  assign: (path) => expiredEvents.push("redirect:" + path),
};
globalThis.dispatchEvent = (event) => {
  expiredEvents.push(event.type);
  return true;
};

let refreshCalls = 0;
let rejectRefresh = false;
let rejectRetriedRequests = false;
globalThis.fetch = async (input, init) => {
  const request = input instanceof Request ? input : new Request(new URL(input, "https://school.example"), init);
  const url = new URL(request.url);
  if (url.pathname === "/backend/auth/refresh") {
    refreshCalls += 1;
    if (rejectRefresh) return new Response("unauthorized", { status: 401 });
    return Response.json({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });
  }
  if (url.pathname === "/backend/public/ping") return Response.json({ public: true });
  const authorization = request.headers.get("Authorization");
  if (authorization !== "Bearer new-access" || rejectRetriedRequests) {
    return new Response("unauthorized", { status: 401 });
  }
  return Response.json({ ok: true });
};

installAuthFetch();

// Two concurrent expired requests must rotate once, then both retry with the
// new access token. This models two components in one tab; the lock serializes
// tabs using the same browser origin as well.
const [first, second] = await Promise.all([
  fetch("https://school.example/backend/me", {
    headers: { Authorization: "Bearer old-access" },
  }),
  fetch("https://school.example/backend/admin/users", {
    headers: { Authorization: "Bearer old-access" },
  }),
]);
assert.equal(first.status, 200);
assert.equal(second.status, 200);
assert.equal(refreshCalls, 1);
assert.equal(values.get("access_token"), "new-access");
assert.equal(values.get("refresh_token"), "new-refresh");

// A public request without a bearer token must not trigger refresh.
const publicResponse = await fetch("https://school.example/backend/public/ping");
assert.equal(publicResponse.status, 200);
assert.equal(refreshCalls, 1);

// A retried request that still receives 401 is not refreshed a second time.
rejectRetriedRequests = true;
const stillUnauthorized = await fetch("https://school.example/backend/me", {
  headers: { Authorization: "Bearer new-access" },
});
assert.equal(stillUnauthorized.status, 401);
assert.equal(refreshCalls, 2);
rejectRetriedRequests = false;

// An invalid refresh token clears the local session and signals the app to
// return the user to the appropriate login route.
values.set("access_token", "expired-access");
values.set("refresh_token", "invalid-refresh");
rejectRefresh = true;
const expired = await fetch("https://school.example/backend/me", {
  headers: { Authorization: "Bearer expired-access" },
});
assert.equal(expired.status, 401);
assert.equal(values.has("access_token"), false);
assert.equal(values.has("refresh_token"), false);
assert.ok(expiredEvents.includes("stonez:session-expired"));

console.log("Race-safe authenticated fetch regression checks passed.");
