import { expect, productWithPrices, test } from "./fixtures";

// Limits and demo-data marks follow the API (GET /v1/meta/limits and the
// demo_share fields), so these tests read the API first and check the page
// says the same. They are skipped against an older API without /v1/meta/limits.

type Limits = { compare_max_lines: number; rfq: { max_open: number; min_days: number; max_days: number } };

async function limits(request: import("@playwright/test").APIRequestContext): Promise<Limits> {
  const res = await request.get("/api/v1/meta/limits");
  expect(res.ok(), "the API serves /v1/meta/limits").toBeTruthy();
  return res.json();
}

test.beforeEach(async ({ request }) => {
  const res = await request.get("/api/v1/meta/limits");
  test.skip(res.status() === 404, "the API is older than /v1/meta/limits and the demo_share fields");
});

test("the comparison page counts lines against the API's limit", async ({ page, request }) => {
  const { compare_max_lines: max } = await limits(request);
  await page.goto("/solishtirish?products=pomidor");
  await expect(page.getByText(new RegExp(`Chiziqlar: 1 / ${max}\\b`))).toBeVisible();
});

test("the comparison chart marks demo data with its share", async ({ page, request }) => {
  const res = await request.get("/api/v1/analytics/compare", { params: { products: "pomidor", trade: "ulgurji", period: "90d" } });
  expect(res.ok()).toBeTruthy();
  const { demo_share: share } = (await res.json()) as { demo_share: number };
  await page.goto("/solishtirish?products=pomidor");
  await expect(page.locator("main svg").first()).toBeVisible();
  const marks = page.locator("main [data-demo-share]");
  if (share > 0) {
    await expect(marks.first()).toBeVisible();
    await expect(marks.first()).toContainText(`${Math.max(1, Math.round(share * 100))}%`);
  } else {
    await expect(marks).toHaveCount(0);
  }
});

test("the analysis page marks demo history on its charts", async ({ page, request }) => {
  const product = await productWithPrices(request);
  const res = await request.get(`/api/v1/products/${product.slug}/analytics`);
  expect(res.ok()).toBeTruthy();
  const { quality, forecast } = (await res.json()) as {
    quality: { demo_share: number; history_demo_share: number };
    forecast?: { demo_share: number };
  };
  await page.goto(`/mahsulot/${product.slug}/tahlil`);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(product.name);
  const marks = page.locator("main [data-demo-share]");
  const anyDemo = quality.demo_share > 0 || quality.history_demo_share > 0 || (forecast?.demo_share ?? 0) > 0;
  if (!anyDemo) {
    await expect(marks).toHaveCount(0);
    return;
  }
  // The price chart's card carries the mark of the shown history.
  const chart = page.locator("section", { has: page.getByRole("heading", { name: "Narx tarixi" }) });
  if (quality.demo_share > 0) await expect(chart.locator("[data-demo-share]").first()).toBeVisible();
  expect(await marks.count()).toBeGreaterThan(1);
  // Every mark says how much, as a percentage.
  for (const text of await marks.allInnerTexts()) expect(text).toMatch(/%/);
});
