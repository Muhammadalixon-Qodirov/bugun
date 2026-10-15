import { usePathname, useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useCallback, useMemo } from "react";
import { toast } from "sonner";

import { useLimits } from "@/components/limits";
import type { ApiError } from "@/lib/api/gen/types.gen";
import { formatSom } from "@/lib/labels";
import { MB, mega } from "@/lib/limits";

/** What a failed API call carries, whatever shape the failure arrived in. */
export type Problem = {
  status: number;
  /** Stable code from the API (ApiError.code), or "network" when it was unreachable. */
  code?: string;
  /** The API's Uzbek fallback text. */
  detail?: string;
  fields: { field: string; code: string; params: Record<string, unknown>; fallback?: string }[];
};

/** Reads a failure in any shape; no translation, so it works anywhere. */
export function problemOf(err: unknown): Problem {
  // fetch() rejects with a TypeError when the network is down or the API is unreachable.
  if (err instanceof TypeError) return { status: 0, code: "network", fields: [] };
  if (typeof err === "string") return { status: 500, fields: [] };
  if (!err || typeof err !== "object") return { status: 500, fields: [] };
  const e = err as Partial<ApiError>;
  const fields = (e.errors ?? []).flatMap((f) =>
    f.location?.startsWith("body.") ? [{ field: f.location.slice(5), code: f.code, params: f.params ?? {}, fallback: f.message }] : [],
  );
  return { status: typeof e.status === "number" ? e.status : 500, code: e.code, detail: e.detail, fields };
}

/** What failed, for "Couldn't … Please try again." when nothing more specific is known. */
export type FailedAction = "generic" | "save" | "delete" | "change" | "check" | "renew" | "contact" | "deletePhoto";

/**
 * Error texts in the visitor's language, from the API's stable codes. A lapsed
 * session gets a "Sign in" button on its toast that returns to this page.
 */
export function useApiErrors() {
  const t = useTranslations("errors");
  const locale = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const limits = useLimits();

  // The API's current limit behind each "too many / too large" code, for its message.
  const codeValues = useMemo<Record<string, Record<string, number>>>(
    () => ({
      photo_limit: { max: limits.photos.max_per_listing },
      photo_too_large: { mb: mega(limits.photos.max_bytes, MB), mp: mega(limits.photos.max_pixels, 1_000_000) },
      rfq_limit: { max: limits.rfq.max_open },
      too_many_watches: { max: limits.watches_max },
      too_many_lines: { max: limits.compare_max_lines },
    }),
    [limits],
  );

  // API codes are dynamic; t.has() checks them before they are used as keys.
  type Key = Parameters<typeof t>[0];

  // Whole numbers are grouped like prices ("4 500"); fractions keep their decimals ("12,5").
  const param = useCallback(
    (v: unknown) =>
      typeof v !== "number" ? String(v) : Number.isInteger(v) ? formatSom(v) : String(Math.round(v * 100) / 100).replace(".", locale === "en" ? "." : ","),
    [locale],
  );

  const fieldMessage = useCallback(
    ({ code, params, fallback }: Problem["fields"][number]) => {
      const values = Object.fromEntries(Object.entries(params).map(([k, v]) => [k, param(v)]));
      const key = `fields.${code}` as Key;
      return t.has(key) ? t(key, values) : (fallback ?? t("fields.invalid"));
    },
    [t, param],
  );

  const message = useCallback(
    (err: unknown, action: FailedAction = "generic") => {
      const p = problemOf(err);
      // One bad field: say what it is rather than "check the red fields".
      if (p.code === "validation_failed" && p.fields.length === 1) return fieldMessage(p.fields[0]);
      const code = `codes.${p.code}` as Key;
      if (p.code && t.has(code)) return t(code, codeValues[p.code]);
      if (p.status === 401) return t("codes.unauthorized");
      if (p.status === 429) return t("codes.rate_limited");
      if (p.status >= 500) return t("codes.internal");
      return t(`failed.${action}`);
    },
    [t, fieldMessage, codeValues],
  );

  const fields = useCallback((err: unknown) => problemOf(err).fields.map((f) => ({ field: f.field, message: fieldMessage(f) })), [fieldMessage]);

  const show = useCallback(
    (err: unknown, action?: FailedAction) => {
      const expired = problemOf(err).status === 401;
      toast.error(message(err, action), {
        action: expired ? { label: t("signIn"), onClick: () => router.push(`/kirish?keyin=${encodeURIComponent(pathname)}`) } : undefined,
      });
    },
    [message, router, pathname, t],
  );

  return useMemo(() => ({ show, message, fields }), [show, message, fields]);
}
