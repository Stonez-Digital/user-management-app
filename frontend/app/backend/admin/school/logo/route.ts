const productionApiTarget = "https://stonez-digital-school-api.onrender.com";

function apiTarget() {
  return process.env.API_SERVER_URL || productionApiTarget;
}

async function proxyLogoUpload(request: Request) {
  const upstreamURL = new URL("/admin/school/logo", apiTarget());

  const headers = new Headers(request.headers);
  headers.delete("host");
  headers.delete("content-length");

  const upstream = await fetch(upstreamURL, {
    method: request.method,
    headers,
    body: request.body,
  });

  const responseHeaders = new Headers(upstream.headers);
  responseHeaders.delete("content-encoding");
  responseHeaders.delete("content-length");

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: responseHeaders,
  });
}

export async function POST(request: Request) {
  return proxyLogoUpload(request);
}
