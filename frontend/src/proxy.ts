import { type NextRequest, NextResponse } from "next/server";

import { contentSecurityPolicy } from "@/lib/csp";
import { SESSION_COOKIE } from "@/lib/session";

// Pages for signed-in visitors only. Each page still checks the session itself;
// this only answers visitors without any session cookie before the page starts
// streaming, so they get a real redirect or 404 status instead of a skeleton.
const SIGNED_IN = ["/kabinet", "/mahsulotlarim", "/profil", "/sorovlar", "/narx", "/boshlash"];
const HIDDEN = ["/admin"];
const under = (path: string, roots: string[]) => roots.some((r) => path === r || path.startsWith(`${r}/`));

/**
 * Gives every page a fresh script nonce and its Content-Security-Policy.
 * Next.js reads the nonce from the request's policy and adds it to its own
 * script tags; the root layout hands it to the theme script.
 */
export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const signedOut = !request.cookies.has(SESSION_COOKIE);
  if (signedOut && under(pathname, SIGNED_IN)) {
    return NextResponse.redirect(new URL(`/kirish?keyin=${pathname}`, request.url));
  }

  const nonce = btoa(crypto.randomUUID());
  const policy = contentSecurityPolicy(nonce);
  const headers = new Headers(request.headers);
  headers.set("x-nonce", nonce);
  headers.set("content-security-policy", policy);

  // The admin page does not exist for anyone else (see app/admin/page.tsx).
  const response =
    signedOut && under(pathname, HIDDEN)
      ? NextResponse.rewrite(new URL("/_not-found", request.url), { request: { headers } })
      : NextResponse.next({ request: { headers } });
  response.headers.set("content-security-policy", policy);
  return response;
}

export const config = {
  matcher: [
    {
      // Pages only: not the API proxy, build assets, images or metadata files.
      source: "/((?!api/|_next/static|_next/image|icon\\.png|apple-icon\\.png|logo|robots\\.txt|sitemap\\.xml|opengraph-image).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
