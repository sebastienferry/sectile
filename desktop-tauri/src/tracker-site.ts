export function trackerApiSite(provider: string, site: string): string {
  const trimmed = site.trim().replace(/\/+$/, "");
  if (provider !== "gitlab" || !trimmed) return trimmed;
  const url = new URL(trimmed);
  if (!/^https?:$/.test(url.protocol)) throw new Error("L’adresse GitLab doit utiliser HTTP ou HTTPS.");
  if (!url.pathname.endsWith("/api/v4")) url.pathname = `${url.pathname.replace(/\/+$/, "")}/api/v4`;
  return url.toString().replace(/\/+$/, "");
}
