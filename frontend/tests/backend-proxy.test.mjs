import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const routePath = fileURLToPath(new URL("../app/backend/[...path]/route.ts", import.meta.url));
const routeSource = await readFile(routePath, "utf8");
const compiled = ts.transpileModule(routeSource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText;
const route = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

test("forwards multipart body and preserves its boundary through the proxy", async () => {
  const originalFetch = globalThis.fetch;
  let upstreamRequest;
  globalThis.fetch = async (url, init) => {
    upstreamRequest = { url: String(url), init };
    return new Response(JSON.stringify({ error: "school name is required" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  };

  try {
    const form = new FormData();
    form.append("school_name", "Invalid Proxy Regression");
    form.append("contact_email", "not-an-email");
    const request = new Request("https://portal.example/backend/auth/school-signup", {
      method: "POST",
      body: form,
    });

    const response = await route.POST(request, {
      params: Promise.resolve({ path: ["auth", "school-signup"] }),
    });

    assert.equal(response.status, 400, "upstream validation status should pass through");
    assert.equal(upstreamRequest.url, "https://stonez-digital-school-api.onrender.com/auth/school-signup");
    const contentType = upstreamRequest.init.headers.get("content-type");
    assert.match(contentType, /^multipart\/form-data; boundary=/);
    assert.ok(upstreamRequest.init.body instanceof ArrayBuffer);

    const forwarded = await new Response(upstreamRequest.init.body, {
      headers: { "content-type": contentType },
    }).formData();
    assert.equal(forwarded.get("school_name"), "Invalid Proxy Regression");
    assert.equal(forwarded.get("contact_email"), "not-an-email");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("rejects a streamed request over 6 MB without contacting the API", async () => {
  const originalFetch = globalThis.fetch;
  let fetchCalled = false;
  globalThis.fetch = async () => {
    fetchCalled = true;
    return new Response(null, { status: 200 });
  };

  try {
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new Uint8Array(6 * 1024 * 1024));
        controller.enqueue(new Uint8Array(1));
        controller.close();
      },
    });
    const request = new Request("https://portal.example/backend/auth/school-signup", {
      method: "POST",
      headers: { "content-type": "multipart/form-data; boundary=test-boundary" },
      body: stream,
      duplex: "half",
    });

    const response = await route.POST(request, {
      params: Promise.resolve({ path: ["auth", "school-signup"] }),
    });

    assert.equal(response.status, 413);
    assert.equal(fetchCalled, false, "oversized body must be rejected before upstream fetch");
    assert.match((await response.json()).error, /configured upload limit/);
  } finally {
    globalThis.fetch = originalFetch;
  }
});


test("keeps the default API proxy limit at 1 MiB", async () => {
  const originalFetch = globalThis.fetch;
  let fetchCalled = false;
  globalThis.fetch = async () => {
    fetchCalled = true;
    return new Response(null, { status: 200 });
  };

  try {
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(new Uint8Array(1 * 1024 * 1024));
        controller.enqueue(new Uint8Array(1));
        controller.close();
      },
    });
    const request = new Request("https://portal.example/backend/admin/students", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: stream,
      duplex: "half",
    });
    const response = await route.POST(request, {
      params: Promise.resolve({ path: ["admin", "students"] }),
    });
    assert.equal(response.status, 413);
    assert.equal(fetchCalled, false, "oversized non-signup request must not reach upstream");
  } finally {
    globalThis.fetch = originalFetch;
  }
});
