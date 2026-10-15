import { ogCard, ogSize } from "@/lib/og";
import site from "@/messages/uz/site.json";

// Link previews are fetched without the visitor's language cookie, so they are in Uzbek.
export const alt = site.title;
export const size = ogSize;
export const contentType = "image/png";

export default function Image() {
  const [brand, tagline] = site.title.split(" — ");
  return ogCard({ eyebrow: tagline ?? brand, title: brand, text: site.description });
}
