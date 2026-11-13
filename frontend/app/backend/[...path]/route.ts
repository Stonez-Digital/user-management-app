const API_ORIGIN = "https://stonez-digital-school-api.onrender.com";
const MAX_REQUEST_BODY_BYTES = 1 * 1024 * 1024;
const MAX_SCHOOL_SIGNUP_BODY_BYTES = 6 * 1024 * 1024;

type RouteContext = {
  params: Promise<{ path: string[] }>;
};

class RequestBodyTooLargeError extends Error {
  constructor() {
    super("Request body exceeds the proxy limit");
    this.name = "RequestBodyTooLargeError";
  }
}

async function readBoundedBody(request: Request, maxBytes: number): Promise<ArrayBuffer | undefined> {
  if (request.method === "GET" || request.method === "HEAD" || !request.body) {
    return undefined;
  }

  const contentLength = Number(request.headers.get("content-length"));
  if (Number.isFinite(contentLength) && contentLength > maxBytes) {
    throw new RequestBodyTooLargeError();
  }

  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      totalBytes += value.byteLength;
      if (totalBytes > maxBytes) {
        await reader.cancel();
        throw new RequestBodyTooLargeError();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const body = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body.buffer;
}

/**
 * Same-origin API gateway for the Cloudflare Worker.
 *
 * The frontend is built with vinext for Cloudflare Workers. Keeping this
 * forwarding in an App Router route handler ensures /backend requests are
 * proxied by the deployed Worker rather than depending on Next.js rewrites.
 */
async function proxyBackendRequest(request: Request, context: RouteContext): Promise<Response> {
  const { path } = await context.params;
  const upstreamPath = (path ?? []).map((segment) => encodeURIComponent(segment)).join("/");
  const incomingUrl = new URL(request.url);
  const upstreamUrl = new URL(upstreamPath, API_ORIGIN + "/");
  upstreamUrl.search = incomingUrl.search;

  const headers = new Headers(request.headers);
  // This is a server-to-server request; forwarding the browser Origin would
  // unnecessarily invoke the API's browser CORS policy.
  headers.delete("origin");
  headers.delete("host");
  headers.delete("content-length");
  headers.delete("connection");
  // Never accept browser-supplied proxy authentication headers.
  headers.delete("x-stonez-proxy-timestamp");
  headers.delete("x-stonez-proxy-nonce");
  headers.delete("x-stonez-proxy-signature");

  try {
    // Buffer once within endpoint-specific limits. The exact bytes that are
    // signed below are also the bytes forwarded upstream.
    const maxBodyBytes = request.method === "POST" && upstreamPath === "auth/school-signup"
      ? MAX_SCHOOL_SIGNUP_BODY_BYTES
      : MAX_REQUEST_BODY_BYTES;
    const body = await readBoundedBody(request, maxBodyBytes);

    const runtimeEnv = (globalThis as unknown as {
      process?: { env?: Record<string, string | undefined> };
    }).process?.env;
    const proxySecret = runtimeEnv?.API_PROXY_HMAC_SECRET?.trim();
    if (proxySecret) {
      const timestamp = Math.floor(Date.now() / 1000).toString();
      const nonce = crypto.randomUUID().replace(/-/g, "");
      const bodyBytes = body ? new Uint8Array(body) : new Uint8Array();
      const digest = await crypto.subtle.digest("SHA-256", bodyBytes);
      const digestHex = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
      const canonical = [
        request.method,
        upstreamUrl.pathname + upstreamUrl.search,
        timestamp,
        nonce,
        digestHex,
      ].join("\n");
      const key = await crypto.subtle.importKey(
        "raw",
        new TextEncoder().encode(proxySecret),
        { name: "HMAC", hash: "SHA-256" },
        false,
        ["sign"],
      );
      const signatureBytes = new Uint8Array(
        await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(canonical)),
      );
      let binary = "";
      for (const byte of signatureBytes) binary += String.fromCharCode(byte);
      const signature = btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/g, "");
      headers.set("x-stonez-proxy-timestamp", timestamp);
      headers.set("x-stonez-proxy-nonce", nonce);
      headers.set("x-stonez-proxy-signature", signature);
    }

    const upstream = await fetch(upstreamUrl, {
      method: request.method,
      headers,
      body,
      redirect: "manual",
      cache: "no-store",
    });

    const responseHeaders = new Headers(upstream.headers);
    responseHeaders.delete("connection");
    responseHeaders.delete("transfer-encoding");
    responseHeaders.delete("content-encoding");
    responseHeaders.delete("content-length");

    return new Response(upstream.body, {
      status: upstream.status,
      statusText: upstream.statusText,
      headers: responseHeaders,
    });
  } catch (error) {
    if (error instanceof RequestBodyTooLargeError) {
      return Response.json(
        { error: "Request body exceeds the configured upload limit." },
        { status: 413, headers: { "Cache-Control": "no-store" } },
      );
    }

    // Keep the client response generic while retaining actionable details in
    // Cloudflare Worker logs for diagnosing transport failures.
    console.error("School portal API proxy request failed", {
      path: upstreamPath,
      method: request.method,
      error: error instanceof Error ? error.message : String(error),
    });

    return Response.json(
      { error: "The school portal API is temporarily unavailable. Please try again." },
      { status: 502, headers: { "Cache-Control": "no-store" } },
    );
  }
}

export const GET = proxyBackendRequest;
export const HEAD = proxyBackendRequest;
export const POST = proxyBackendRequest;
export const PUT = proxyBackendRequest;
export const PATCH = proxyBackendRequest;
export const DELETE = proxyBackendRequest;
export const OPTIONS = proxyBackendRequest;
