"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useEffect } from "react";

import { Page } from "@/components/page";
import { Button, buttonVariants } from "@/components/ui/button";
import { captureBoundaryError } from "@/lib/sentry";

export type ErrorProps = { error: Error & { digest?: string }; retry: () => void };

/**
 * What a page shows when it fails to load (the app/**\/error.tsx files). The
 * header and footer stay, so the visitor can retry or go elsewhere.
 */
export function RouteError({ error, retry }: ErrorProps) {
  const t = useTranslations("site");
  useEffect(() => captureBoundaryError(error), [error]);
  return (
    <Page className="grid min-h-[calc(var(--app-height)-4rem)] place-items-center py-16 text-center">
      <div className="max-w-md">
        <p className="eyebrow">{t("error.eyebrow")}</p>
        <h1 className="display mt-3 text-4xl sm:text-5xl">{t("error.title")}</h1>
        <p className="mt-4 leading-relaxed text-muted-foreground">
          {t("error.text")}
        </p>
        <div className="mt-8 flex flex-wrap justify-center gap-2">
          <Button variant="solid" size="pill" onClick={() => retry()} className="h-11 px-6">
            {t("error.retry")}
          </Button>
          <Link href="/" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11 px-6" })}>
            {t("backToMarket")}
          </Link>
        </div>
        {error.digest && <p className="num mt-8 text-xs text-muted-foreground">{t("error.code", { digest: error.digest })}</p>}
      </div>
    </Page>
  );
}
