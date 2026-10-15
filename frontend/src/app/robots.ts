import type { MetadataRoute } from "next";

import { siteUrl } from "@/lib/env";

export default function robots(): MetadataRoute.Robots {
  const base = siteUrl();
  return {
    // Signed-in and sign-in pages have nothing for search engines.
    rules: { userAgent: "*", allow: "/", disallow: ["/admin", "/kabinet", "/profil", "/mahsulotlarim", "/kirish", "/boshlash", "/api/"] },
    sitemap: `${base}/sitemap.xml`,
  };
}
