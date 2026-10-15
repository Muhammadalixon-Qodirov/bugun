import { type APIRequestContext, expect, type Page, test as base } from "@playwright/test";

// React's development build can fail to record a performance mark when a
// server redirect lands; it is a dev-only warning, not an app error.
const ignored = [/cannot have a negative time stamp/];

/**
 * Every test fails on a console error, an uncaught page error or a
 * Content-Security-Policy violation, whatever it was checking.
 */
export const test = base.extend<{ problems: string[] }>({
  problems: [
    async ({ page }, use) => {
      const problems: string[] = [];
      const keep = (text: string) => !ignored.some((r) => r.test(text)) && problems.push(text);
      page.on("console", (m) => {
        if (m.type() !== "error") return;
        // A page answered with 404 on purpose, which Chrome reports for the document itself.
        if (m.text().includes("status of 404") && m.location().url === page.url()) return;
        keep(`console: ${m.text()}`);
      });
      page.on("pageerror", (e) => keep(`page error: ${e.message}`));
      await page.addInitScript(() => {
        document.addEventListener("securitypolicyviolation", (e) =>
          console.error(`CSP violation: ${e.violatedDirective} blocked ${e.blockedURI || "inline"}`),
        );
      });
      await use(problems);
      expect(problems, "console errors, page errors or CSP violations").toEqual([]);
    },
    { auto: true },
  ],
});

export { expect };

type Product = { slug: string; name: string };

/** A product that has enough prices for its page to show them, from the API behind the site's /api proxy. */
export async function productWithPrices(request: APIRequestContext): Promise<Product> {
  const res = await request.get("/api/v1/listings", { params: { per_page: 60 } });
  expect(res.ok()).toBeTruthy();
  const { listings } = (await res.json()) as { listings: { product_slug: string; product_name: string }[] };
  const counts = new Map<string, Product & { n: number }>();
  for (const l of listings) {
    const p = counts.get(l.product_slug) ?? { slug: l.product_slug, name: l.product_name, n: 0 };
    p.n++;
    counts.set(l.product_slug, p);
  }
  const best = [...counts.values()].sort((a, b) => b.n - a.n)[0];
  expect(best, "the API has listings").toBeTruthy();
  return best;
}

/** True when the page is wider than the window, i.e. scrolls sideways. */
export function scrollsSideways(page: Page) {
  return page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1);
}
