const API_ORIGIN = "https://stonez-digital-school-api.onrender.com";

type RouteContext = {
  params: Promise<{ path: string[] }>;
};

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
    const upstream = await fetch(upstreamUrl, {
      method: request.method,
      headers,
      body: request.method === "GET" || request.method === "HEAD" ? undefined : request.body,
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
  } catch {
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
