import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures";

// Sign-in from the Mini App's launch data (components/telegram-app.tsx). The
// real telegram-web-app.js is replaced by a small stand-in, and the API's
// answer by a canned one, so these run offline and never create a user.

const LOGIN = "**/v1/auth/telegram/webapp";

/** Serves a minimal window.Telegram.WebApp instead of telegram.org's script. */
async function fakeTelegram(page: Page, initData: string) {
  await page.route("https://telegram.org/js/telegram-web-app.js", (route) =>
    route.fulfill({
      contentType: "text/javascript",
      body: `(() => {
        const noop = () => {};
        window.Telegram = { WebApp: {
          initData: ${JSON.stringify(initData)}, colorScheme: "light",
          ready: noop, expand: noop, isVersionAtLeast: () => true, disableVerticalSwipes: noop,
          setHeaderColor: noop, setBackgroundColor: noop, setBottomBarColor: noop, onEvent: noop, offEvent: noop,
          BackButton: { show: noop, hide: noop, onClick: noop, offClick: noop },
        } };
      })();`,
    }),
  );
}

/** Counts the Mini App sign-in requests and answers them with `next`. */
async function stubLogin(page: Page, next: string) {
  const bodies: unknown[] = [];
  await page.route(LOGIN, (route) => {
    bodies.push(route.request().postDataJSON());
    return route.fulfill({ json: { next, needs_onboarding: false } });
  });
  return bodies;
}

test("Mini App sign-in does nothing outside Telegram", async ({ page }) => {
  const bodies = await stubLogin(page, "/");
  let telegramScript = false;
  page.on("request", (r) => {
    if (r.url().startsWith("https://telegram.org/")) telegramScript = true;
  });
  await page.goto("/kirish?keyin=/katalog");
  await page.waitForLoadState("networkidle");
  await page.goto("/");
  await page.waitForLoadState("networkidle");
  expect(bodies).toEqual([]);
  expect(telegramScript).toBe(false);
  expect(await page.evaluate(() => "Telegram" in window)).toBe(false);
});

test("the Mini App signs in once from Telegram's launch data", async ({ page }) => {
  await fakeTelegram(page, "query_id=test&user=%7B%7D&hash=test");
  const bodies = await stubLogin(page, "/katalog");

  // The bot's buttons open signed-in pages; signed-out visitors land on the login page first.
  await page.goto("/kirish?keyin=/katalog#tgWebAppData=test&tgWebAppVersion=8.0&tgWebAppPlatform=web");
  await expect(page).toHaveURL(/\/katalog$/);
  expect(bodies).toEqual([{ init_data: "query_id=test&user=%7B%7D&hash=test", return_to: "/katalog" }]);

  // Never again in the same visit, even though the stub set no session.
  await page.reload();
  await page.waitForLoadState("networkidle");
  expect(bodies).toHaveLength(1);
});
