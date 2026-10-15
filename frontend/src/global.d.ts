import type { Locale } from "@/i18n/config";
import type messages from "@/messages/uz";

// Uzbek is the source language: every key used in code must exist in messages/uz,
// and messages/en and messages/ru must have the same keys (npm run i18n:check).
declare module "next-intl" {
  interface AppConfig {
    Locale: Locale;
    Messages: typeof messages;
  }
}
