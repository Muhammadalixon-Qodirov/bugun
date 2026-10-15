import { publicApiUrl } from "@/lib/api/client-config";

/**
 * The photo for link previews (Telegram, OpenGraph) as openGraph fields. A page
 * that sets its own openGraph loses the site's generated image, so without a
 * photo (or in local development, where photo URLs are relative) it is named here.
 */
export function previewImages(key: string | undefined) {
  const url = key ? mediaUrl(key) : "";
  return { images: [url.startsWith("http") ? { url } : { url: "/opengraph-image", width: 1200, height: 630 }] };
}

/** Public URL of a stored listing photo; `thumb` gives the 480px card copy. */
export function mediaUrl(key: string, thumb = false) {
  return `${publicApiUrl}/media/${thumb ? key.replace(/\.jpg$/, "_t.jpg") : key}`;
}
