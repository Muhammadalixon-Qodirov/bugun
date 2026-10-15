import { expect, productWithPrices, scrollsSideways, test } from "./fixtures";

// Tests run signed out and in Uzbek (no cookies), unless a test changes that.

test("home page shows the market", async ({ page }) => {
  const res = await page.goto("/");
  expect(res?.status()).toBe(200);
  await expect(page.locator("header").getByRole("link").first()).toBeVisible();
  await expect(page.locator('main a[href^="/elon/"]').first()).toBeVisible();
});

test("catalog lists products", async ({ page }) => {
  await page.goto("/katalog");
  await expect(page.locator('main a[href^="/elon/"]').first()).toBeVisible();
  expect(await page.locator('main a[href^="/elon/"]').count()).toBeGreaterThan(1);
});

test("product page shows prices", async ({ page, request }) => {
  const product = await productWithPrices(request);
  const res = await page.goto(`/mahsulot/${product.slug}`);
  expect(res?.status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(product.name);
  // A price is digits followed by the "so'm/kg" unit label.
  await expect(page.locator("main p", { hasText: /^\d[\d\s]*so'm\// }).first()).toBeVisible();
});

test("product analysis page renders", async ({ page, request }) => {
  const product = await productWithPrices(request);
  const res = await page.goto(`/mahsulot/${product.slug}/tahlil`);
  expect(res?.status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(product.name);
  await expect(page.locator("main svg").first()).toBeVisible();
});

for (const path of ["/kabinet", "/mahsulotlarim"]) {
  test(`${path} sends signed-out visitors to sign in`, async ({ page }) => {
    const res = await page.request.get(path, { maxRedirects: 0 });
    expect(res.status()).toBe(307);
    await page.goto(path);
    await expect(page).toHaveURL(new RegExp(`/kirish\\?keyin=${path}$`));
  });
}

test("admin page does not exist for signed-out visitors", async ({ page }) => {
  const res = await page.goto("/admin");
  expect(res?.status()).toBe(404);
  await expect(page.getByText("404", { exact: true })).toBeVisible();
});

test("language switch changes the interface language", async ({ page, context }) => {
  await page.goto("/");
  await expect(page.locator("html")).toHaveAttribute("lang", "uz");
  await page.getByRole("button", { name: "Til" }).click();
  await page.getByRole("menuitem", { name: "English" }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.getByRole("button", { name: "Language" })).toBeVisible();
  expect((await context.cookies()).find((c) => c.name === "NEXT_LOCALE")?.value).toBe("en");
});

test.describe("security headers", () => {
  test("pages send a Content-Security-Policy with a nonce", async ({ request }) => {
    const res = await request.get("/");
    const headers = res.headers();
    expect(headers["content-security-policy"]).toMatch(/script-src [^;]*'nonce-[^']+'/);
    expect(headers["content-security-policy"]).toContain("frame-ancestors 'self' https://web.telegram.org");
    expect(headers["x-content-type-options"]).toBe("nosniff");
    expect(headers["referrer-policy"]).toBe("strict-origin-when-cross-origin");
  });
});

test.describe("fits a 390px phone", () => {
  test.skip(({ viewport }) => (viewport?.width ?? 0) > 390, "mobile project only");
  for (const path of ["/", "/katalog", "/statistika", "/xarita", "/solishtirish", "/kirish"]) {
    test(`${path} does not scroll sideways`, async ({ page }) => {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      expect(await scrollsSideways(page)).toBe(false);
    });
  }
  test("product pages do not scroll sideways", async ({ page, request }) => {
    const product = await productWithPrices(request);
    for (const path of [`/mahsulot/${product.slug}`, `/mahsulot/${product.slug}/tahlil`]) {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      expect(await scrollsSideways(page), path).toBe(false);
    }
  });
});

test("the Telegram Mini App flag script runs under the policy", async ({ page }) => {
  // Launch data "test" is no real sign-in: keep the Mini App login from calling the API.
  await page.route("**/v1/auth/telegram/webapp", (route) => route.fulfill({ json: { next: "/", needs_onboarding: false } }));
  await page.goto("/#tgWebAppData=test&tgWebAppVersion=7.10&tgWebAppPlatform=web");
  await expect(page.locator("html")).toHaveClass(/tg-webapp/);
});

test("the product page shows the exchange price when the API has one", async ({ page, request }) => {
  const catalog = await request.get("/api/v1/catalog");
  expect(catalog.ok()).toBeTruthy();
  const { products } = (await catalog.json()) as { products: { slug: string }[] };
  let found: { slug: string; price: number } | undefined;
  for (const { slug } of products) {
    const res = await request.get(`/api/v1/products/${slug}/references`, { params: { region: 0, trade: "ulgurji" } });
    test.skip(res.status() === 404 && slug === products[0].slug, "the API is older than /v1/products/{slug}/references");
    if (!res.ok()) continue;
    const { references } = (await res.json()) as { references: { price_som: number }[] };
    if (references.length > 0) {
      found = { slug, price: references[0].price_som };
      break;
    }
  }
  test.skip(!found, "the API has no reference prices yet");

  await page.goto(`/mahsulot/${found!.slug}`);
  const line = page.getByTestId("reference-prices");
  await expect(line).toBeVisible();
  await expect(line).toContainText(/UzEx|Rasmiy/);
});
