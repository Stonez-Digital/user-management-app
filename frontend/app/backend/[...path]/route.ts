const API_ORIGIN = "https://stonez-digital-school-api.onrender.com";
const MAX_PROXY_BODY_BYTES = 6 * 1024 * 1024;

type RouteContext = {
  params: Promise<{ path: string[] }>;
};

class RequestBodyTooLargeError extends Error {
  constructor() {
    super("Request body exceeds the proxy limit");
    this.name = "RequestBodyTooLargeError";
  }
}

async function readBoundedBody(request: Request): Promise<ArrayBuffer | undefined> {
  if (request.method === "GET" || request.method === "HEAD" || !request.body) {
    return undefined;
  }

  const contentLength = Number(request.headers.get("content-length"));
  if (Number.isFinite(contentLength) && contentLength > MAX_PROXY_BODY_BYTES) {
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
      if (totalBytes > MAX_PROXY_BODY_BYTES) {
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

  try {
    // Enforce the limit while reading, not just from Content-Length (which
    // may be absent or inaccurate). The original multipart Content-Type,
    // including its boundary, remains untouched.
    const body = await readBoundedBody(request);

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
        { error: "Request body exceeds the 6 MB upload limit." },
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
