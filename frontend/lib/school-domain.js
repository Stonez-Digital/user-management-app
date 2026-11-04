const DEFAULT_BASE_DOMAIN = "stonezdigital.com.ng";
const RESERVED_SUBDOMAINS = new Set(["app", "api", "www", "mail", "smtp", "ftp", "admin", "autodiscover"]);

export function schoolSlugFromHostname(hostname, baseDomain = DEFAULT_BASE_DOMAIN) {
  const host = String(hostname || "").trim().toLowerCase().replace(/:\d+$/, "").replace(/\.$/, "");
  const normalizedBase = String(baseDomain).trim().toLowerCase().replace(/^\./, "").replace(/\.$/, "");
  const suffix = "." + normalizedBase;
  if (!host.endsWith(suffix)) return null;

  const label = host.slice(0, -suffix.length);
  if (!label || label.includes(".") || RESERVED_SUBDOMAINS.has(label)) return null;
  if (!/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label)) return null;
  return label;
}

export function schoolPathForHostname(hostname, pathname, search = "", baseDomain = DEFAULT_BASE_DOMAIN) {
  const slug = schoolSlugFromHostname(hostname, baseDomain);
  if (!slug) return null;

  const path = String(pathname || "/");
  let suffix = path;
  if (path === "/school") {
    suffix = "";
  } else if (path.startsWith("/school/")) {
    const [, , pathSlug, ...rest] = path.split("/");
    if (pathSlug === slug) return null;
    suffix = rest.length ? "/" + rest.join("/") : "";
  }

  const query = String(search || "");
  return "/school/" + slug + (suffix === "/" ? "" : suffix) + (query.startsWith("?") || !query ? query : "?" + query);
}
