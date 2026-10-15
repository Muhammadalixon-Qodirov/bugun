import { expect, productWithPrices, test } from "./fixtures";

// The assistant's replies come from POST /v1/chat. It is mocked here, so the
// page is checked the same way against an API with or without the endpoint.
const answer = {
  session_id: "s-1",
  kind: "answer",
  text: "Samarqand viloyatida piyoz ulgurji o'rtacha 4 500 so'm/kg.",
  status: "ok",
  mode: "rules",
  lang: "uz",
  table: { columns: ["area", "median", "observations"], rows: [["Samarqand viloyati", 4500, 12]] },
  sources: [
    {
      tool: "price_stats",
      params: { product: "piyoz", region: "Samarqand viloyati", trade: "ulgurji" },
      data_from: "2026-09-19",
      data_to: "2026-09-26",
      observations: 12,
      demo_share: 0.4,
      enough: true,
    },
  ],
};

type Sent = { message?: string; lang?: string; session_id?: string };

async function mockChat(page: import("@playwright/test").Page, replies: object[]) {
  const sent: Sent[] = [];
  await page.route("**/v1/chat", async (route) => {
    sent.push(route.request().postDataJSON());
    await route.fulfill({ json: replies[Math.min(sent.length, replies.length) - 1] });
  });
  return sent;
}

test("the assistant answers, keeps the conversation and starts a new one", async ({ page }) => {
  const sent = await mockChat(page, [answer, { ...answer, text: "Andijonda o'rtacha 4 300 so'm/kg." }]);
  const res = await page.goto("/tahlilchi");
  expect(res?.status()).toBe(200);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

  const input = page.getByTestId("chat-input");
  await input.fill("Samarqandda piyoz qancha?");
  await input.press("Enter");

  const shown = page.getByTestId("chat-answer").first();
  await expect(shown).toContainText(answer.text);
  expect(sent[0]).toEqual({ message: "Samarqandda piyoz qancha?", lang: "uz" });
  // The grey line under the answer: prices, dates and the demo mark.
  await expect(shown.getByTestId("chat-source-line")).toContainText("12 ta narx asosida");
  await expect(shown.locator("[data-demo-share]").first()).toContainText("40%");
  // The table and the sources open on request.
  await shown.getByRole("button", { name: /Jadval/ }).click();
  await expect(shown.getByRole("columnheader", { name: "O'rtacha" })).toBeVisible();
  await expect(shown.getByRole("cell", { name: /^4\s500$/ })).toBeVisible();
  await shown.getByRole("button", { name: "Manbalar" }).click();
  await expect(shown).toContainText("Hozirgi narx");

  // A follow-up goes with the session id.
  await input.fill("Andijonda-chi?");
  await page.getByTestId("chat-send").click();
  await expect(page.getByTestId("chat-answer")).toHaveCount(2);
  expect(sent[1]).toEqual({ message: "Andijonda-chi?", lang: "uz", session_id: "s-1" });

  // The conversation survives a reload; "New chat" clears it.
  await page.reload();
  await expect(page.getByTestId("chat-user")).toHaveCount(2);
  await page.getByTestId("chat-new").click();
  await expect(page.getByTestId("chat-user")).toHaveCount(0);
  await page.reload();
  await expect(page.getByTestId("chat-user")).toHaveCount(0);
});

test("a clarifying question offers buttons that answer it", async ({ page }) => {
  const clarify = { ...answer, kind: "clarify", text: "Qaysi savdo turi kerak?", options: ["Ulgurji", "Chakana"], table: undefined, sources: [] };
  const sent = await mockChat(page, [clarify, answer]);
  await page.goto("/tahlilchi");
  // An example question is sent as it is.
  await page.getByRole("button", { name: /qancha\?$/ }).first().click();
  await expect(page.getByTestId("chat-option")).toHaveCount(2);
  await page.getByTestId("chat-option").filter({ hasText: "Ulgurji" }).click();
  await expect(page.getByTestId("chat-answer")).toHaveCount(2);
  expect(sent[1]).toEqual({ message: "Ulgurji", lang: "uz", session_id: "s-1" });
  await expect(page.getByTestId("chat-option").first()).toBeDisabled();
});

test("answers are plain text and proposed actions wait for confirmation", async ({ page }) => {
  const withActions = {
    ...answer,
    text: "<b>Piyoz</b> 4 500 so'm\nikkinchi qator",
    actions: [
      { type: "watch_price", label: "Piyoz narxi tushsa xabar berish", params: { product_id: 1, province_id: 0, trade: "ulgurji" } },
      { type: "create_request", label: "So'rov yuborish: Piyoz, 100 kg", params: { product_id: 1, product: "piyoz", quantity: 100, unit: "kg", days: 3 } },
    ],
  };
  await mockChat(page, [withActions]);
  let watched = false;
  await page.route("**/v1/watches", (route) => {
    watched = true;
    return route.fulfill({ json: { watches: [] } });
  });
  await page.goto("/tahlilchi");
  await page.getByTestId("chat-input").fill("piyoz");
  await page.getByTestId("chat-input").press("Enter");
  const shown = page.getByTestId("chat-answer");
  // Never HTML: the tags show as they are.
  await expect(shown).toContainText("<b>Piyoz</b>");
  await expect(shown.locator("b")).toHaveCount(0);

  await page.getByTestId("chat-action-watch_price").click();
  await expect(page.getByRole("dialog")).toContainText("Narxni kuzatish");
  await page.getByRole("button", { name: "Bekor qilish" }).click();
  expect(watched).toBe(false);

  // Signed out, confirming a request goes to sign-in and comes back to the filled form.
  await page.getByTestId("chat-action-create_request").click();
  await page.getByTestId("chat-action-confirm").click();
  await expect(page).toHaveURL(/\/kirish\?keyin=%2Fsorovlar%3Fmahsulot%3Dpiyoz%26miqdor%3D100%26kun%3D3$/);
  expect(watched).toBe(false);
});

test("the assistant explains a rate limit and can send again", async ({ page, problems }) => {
  let calls = 0;
  await page.route("**/v1/chat", (route) => {
    calls++;
    return calls === 1
      ? route.fulfill({ status: 429, json: { status: 429, code: "rate_limited", title: "Too Many Requests" } })
      : route.fulfill({ json: answer });
  });
  await page.goto("/tahlilchi");
  await page.getByTestId("chat-input").fill("piyoz");
  await page.getByTestId("chat-input").press("Enter");
  await expect(page.getByRole("alert").filter({ hasText: "Juda ko'p savol" })).toBeVisible();
  await page.getByRole("button", { name: "Qayta yuborish" }).click();
  await expect(page.getByTestId("chat-answer")).toContainText(answer.text);
  await expect(page.getByTestId("chat-user")).toHaveCount(1);
  // The browser logs the refused request itself; that one is expected here.
  const i = problems.findIndex((p) => p.includes("status of 429"));
  if (i >= 0) problems.splice(i, 1);
});

test("a switched-off assistant says it is not available", async ({ page, problems }) => {
  await page.route("**/v1/chat", (route) => route.fulfill({ status: 503, json: { status: 503, code: "feature_off", title: "Service Unavailable" } }));
  await page.goto("/tahlilchi");
  await page.getByTestId("chat-input").fill("piyoz");
  await page.getByTestId("chat-input").press("Enter");
  await expect(page.getByText("Yordamchi hozircha mavjud emas")).toBeVisible();
  const i = problems.findIndex((p) => p.includes("status of 503"));
  if (i >= 0) problems.splice(i, 1);
});

test("the analysis page hands its question to the assistant", async ({ page, request }) => {
  const product = await productWithPrices(request);
  const sent = await mockChat(page, [answer]);
  await page.goto(`/mahsulot/${product.slug}/tahlil`);
  await page.getByRole("link", { name: "Savol bering" }).click();
  await expect(page.getByTestId("chat-answer")).toBeVisible();
  expect(sent[0].message?.toLowerCase()).toContain(product.name.toLowerCase());
  // The question is not sent again after a reload.
  await expect(page).toHaveURL(/\/tahlilchi$/);
  await page.reload();
  await expect(page.getByTestId("chat-user")).toHaveCount(1);
  expect(sent).toHaveLength(1);
});

test("the admin assistant tab does not exist for signed-out visitors", async ({ page }) => {
  const res = await page.goto("/admin?tab=assistant");
  expect(res?.status()).toBe(404);
});

test("the admin model usage tab does not exist for signed-out visitors", async ({ page }) => {
  const res = await page.goto("/admin?tab=ai");
  expect(res?.status()).toBe(404);
});

// The price page is for signed-in entrepreneurs. With such a session cookie in
// E2E_SELLER_SESSION, check that the voice and photo buttons follow
// GET /v1/ai/features (an API without it, answering 404, has them off).
test("voice and photo buttons follow the API's features", async ({ page, context, request }) => {
  const session = process.env.E2E_SELLER_SESSION;
  test.skip(!session, "needs an entrepreneur session in E2E_SELLER_SESSION");
  const res = await request.get("/api/v1/ai/features");
  const features = res.ok() ? ((await res.json()) as { voice: boolean; photo: boolean }) : { voice: false, photo: false };
  const host = new URL(process.env.E2E_BASE_URL ?? "http://localhost:3100").hostname;
  await context.addCookies([{ name: "bugun_session", value: session!, domain: host, path: "/" }]);
  await page.goto("/narx");
  await expect(page.getByRole("textbox", { name: "Narx matni" })).toBeVisible();
  await expect(page.getByTestId("voice-button")).toHaveCount(features.voice ? 1 : 0);
  await expect(page.getByTestId("photo-button")).toHaveCount(features.photo ? 1 : 0);
});

test("the price page sends signed-out visitors to sign in", async ({ page }) => {
  await page.goto("/narx");
  await expect(page).toHaveURL(/\/kirish\?keyin=\/narx$/);
});

test("the corner assistant opens on any page and shares the conversation with /tahlilchi", async ({ page }) => {
  const sent = await mockChat(page, [answer]);
  await page.goto("/katalog");
  await page.getByTestId("chat-launcher").click();
  const widget = page.getByTestId("chat-widget");
  await expect(widget).toBeVisible();
  await widget.getByTestId("chat-input").fill("Samarqandda piyoz qancha?");
  await widget.getByTestId("chat-input").press("Enter");
  await expect(widget.getByTestId("chat-answer")).toContainText(answer.text);
  expect(sent).toHaveLength(1);

  // Closing keeps the conversation; opening again shows it.
  await widget.getByTestId("chat-close").click();
  await expect(widget).toBeHidden();
  await page.getByTestId("chat-launcher").click();
  await expect(widget.getByTestId("chat-user")).toHaveCount(1);

  // The full page continues the same conversation, and hides the corner button.
  await widget.getByRole("link", { name: "Katta oynada ochish" }).click();
  await expect(page).toHaveURL(/\/tahlilchi$/);
  await expect(page.locator("main").getByTestId("chat-user")).toHaveCount(1);
  await expect(page.getByTestId("chat-widget")).toBeHidden();
  await expect(page.getByTestId("chat-launcher")).toHaveCount(0);
});

test("Escape closes the corner assistant", async ({ page, isMobile }) => {
  test.skip(!!isMobile, "no keyboard on phones");
  await page.goto("/");
  await page.getByTestId("chat-launcher").click();
  await expect(page.getByTestId("chat-input")).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("chat-widget")).toBeHidden();
  await expect(page.getByTestId("chat-launcher")).toBeVisible();
});
