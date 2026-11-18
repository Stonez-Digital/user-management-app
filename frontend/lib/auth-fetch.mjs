const LOCK_NAME = "stonez-digital-auth-refresh";
const SESSION_EXPIRED_EVENT = "stonez:session-expired";
const INSTALL_KEY = "__stonezAuthFetchInstalled";

function clearSession() {
  localStorage.removeItem("access_token");
  localStorage.removeItem("refresh_token");
  localStorage.removeItem("school_code");
}

function expireSession() {
  clearSession();
  window.dispatchEvent(new CustomEvent(SESSION_EXPIRED_EVENT));
}

function isAuthEndpoint(url) {
  return /\/backend\/auth\/(login|refresh|logout|logout-all)(?:[/?#]|$)/.test(url);
}

function currentAccessToken() {
  try {
    return localStorage.getItem("access_token");
  } catch {
    return null;
  }
}

async function rotateToken(originalFetch, failedToken) {
  const locks = globalThis.navigator?.locks;
  // Without a cross-tab lock, rotating a single-use refresh token could
  // revoke the family when two tabs refresh concurrently. Fail closed.
  if (!locks || typeof locks.request !== "function") return null;

  return locks.request(LOCK_NAME, { mode: "exclusive" }, async () => {
    const latestToken = currentAccessToken();
    // Another tab may already have completed rotation while this tab waited.
    if (latestToken && latestToken !== failedToken) return latestToken;

    let refreshToken;
    try {
      refreshToken = localStorage.getItem("refresh_token");
    } catch {
      return null;
    }
    if (!refreshToken) {
      expireSession();
      return null;
    }

    let response;
    try {
      response = await originalFetch("/backend/auth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
        cache: "no-store",
      });
    } catch {
      // A network failure is not proof that the refresh token is invalid.
      return null;
    }

    if (response.status === 401 || response.status === 403) {
      expireSession();
      return null;
    }
    if (!response.ok) return null;

    let payload;
    try {
      payload = await response.clone().json();
    } catch {
      expireSession();
      return null;
    }
    if (typeof payload?.access_token !== "string" ||
        typeof payload?.refresh_token !== "string" ||
        !payload.access_token ||
        !payload.refresh_token) {
      expireSession();
      return null;
    }

    localStorage.setItem("access_token", payload.access_token);
    localStorage.setItem("refresh_token", payload.refresh_token);
    window.dispatchEvent(new CustomEvent("stonez:tokens-refreshed"));
    return payload.access_token;
  });
}

export function installAuthFetch() {
  if (typeof window === "undefined" || window[INSTALL_KEY]) return;
  window[INSTALL_KEY] = true;

  const originalFetch = window.fetch.bind(window);

  window.fetch = async (input, init) => {
    let request;
    try {
      request = new Request(input, init);
    } catch {
      return originalFetch(input, init);
    }

    const url = request.url;
    const isBackend = new URL(url).origin === window.location.origin &&
      new URL(url).pathname.startsWith("/backend/");
    const initialAuthorization = request.headers.get("Authorization") || "";
    const isBearerRequest = /^Bearer\s+\S+/i.test(initialAuthorization);

    if (!isBackend || !isBearerRequest || isAuthEndpoint(new URL(url).pathname)) {
      return originalFetch(request);
    }

    const tokenAtStart = currentAccessToken();
    const headers = new Headers(request.headers);
    if (tokenAtStart) headers.set("Authorization", "Bearer " + tokenAtStart);
    const authenticatedRequest = new Request(request, { headers });
    const retryTemplate = authenticatedRequest.clone();
    const response = await originalFetch(authenticatedRequest);

    if (response.status !== 401) return response;

    const failedToken = tokenAtStart || initialAuthorization.replace(/^Bearer\s+/i, "");
    const refreshedToken = await rotateToken(originalFetch, failedToken);
    if (!refreshedToken) return response;

    const retryHeaders = new Headers(retryTemplate.headers);
    retryHeaders.set("Authorization", "Bearer " + refreshedToken);
    const retryRequest = new Request(retryTemplate, { headers: retryHeaders });
    // Exactly one retry: a second 401 is returned to the caller unchanged.
    return originalFetch(retryRequest);
  };
}
