export class ApiError extends Error {
  constructor(message, { status = 0, code = "request_failed", details = null } = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

function parseResponseBody(raw) {
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

function formatValidationDetails(details) {
  if (!Array.isArray(details)) return "";
  return details
    .map((item) => {
      if (!item || typeof item !== "object") return "";
      const field = typeof item.field === "string" ? item.field : "";
      const rule = typeof item.rule === "string" ? item.rule : "";
      return [field, rule].filter(Boolean).join(": ");
    })
    .filter(Boolean)
    .join(", ");
}

export async function apiRequest(path, options = {}) {
  const headers = new Headers(options.headers || {});
  const isFormData = typeof FormData !== "undefined" && options.body instanceof FormData;
  if (!headers.has("Content-Type") && !isFormData) {
    headers.set("Content-Type", "application/json");
  }

  let token = null;
  try {
    token = globalThis.localStorage?.getItem("access_token") || null;
  } catch {
    token = null;
  }
  if (token && !headers.has("Authorization")) {
    headers.set("Authorization", "Bearer " + token);
  }

  const response = await fetch("/backend" + path, {
    ...options,
    headers,
  });

  const payload = parseResponseBody(await response.text());
  if (response.status === 401) {
    throw new ApiError("Session expired", {
      status: 401,
      code: "session_expired",
      details: payload?.error?.details ?? null,
    });
  }

  if (!response.ok) {
    const error = payload && typeof payload === "object" ? payload.error : null;
    const baseMessage = typeof error?.message === "string"
      ? error.message
      : typeof error === "string"
        ? error
        : typeof payload === "string" && payload
          ? payload
          : "Request failed";
    const detailMessage = formatValidationDetails(error?.details);
    throw new ApiError(
      detailMessage ? baseMessage + " (" + detailMessage + ")" : baseMessage,
      {
        status: response.status,
        code: typeof error?.code === "string" ? error.code : "request_failed",
        details: error?.details ?? null,
      },
    );
  }

  return payload;
}
