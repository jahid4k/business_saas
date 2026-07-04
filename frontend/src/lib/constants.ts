// src/lib/constants.ts

// Single source of truth for the backend's origin. Used both by the Axios
// client (lib/api.ts) and by resolveAssetUrl below — previously this was
// inlined directly in lib/api.ts with no way for anything else to reuse it.
export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/**
 * Some backend endpoints (currently: SafeUser.photoURL from the avatar
 * upload handler) return a root-relative path — e.g. "/uploads/avatars/
 * <uuid>.jpg" — relative to the API server, not to whatever origin the
 * value is eventually rendered from. A plain `<img src={photoURL}>` in the
 * frontend resolves that path against the *frontend's* origin instead,
 * which 404s since Next.js doesn't serve anything at /uploads.
 *
 * This normalizes such a path into a full, browser-loadable URL. Already-
 * absolute URLs (http/https) are passed through unchanged, so this stays
 * correct if a future storage backend (S3, a CDN) starts returning full
 * URLs instead of relative paths.
 */
export function resolveAssetUrl(path?: string | null): string | null {
  if (!path) return null;
  if (/^https?:\/\//i.test(path)) return path;
  return `${API_BASE_URL}${path.startsWith("/") ? path : `/${path}`}`;
}
