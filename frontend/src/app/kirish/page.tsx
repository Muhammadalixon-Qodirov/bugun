import { Lock, Zap } from "lucide-react";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { LogoMark } from "@/components/logo";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { TelegramIcon } from "@/components/telegram-icon";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { buttonVariants } from "@/components/ui/button";
import { publicApiUrl } from "@/lib/api/client-config";
import { getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth.login");
  return { title: t("metaTitle") };
}

const errorCodes = ["bekor", "muddat", "telegram", "sozlanmagan"] as const;

function safePath(p: string | undefined) {
  return p && p.startsWith("/") && !p.startsWith("//") ? p : "/";
}

export default async function LoginPage({ searchParams }: PageProps<"/kirish">) {
  const params = await searchParams;
  const t = await getTranslations("auth.login");
  const next = safePath(typeof params.keyin === "string" ? params.keyin : undefined);
  const code = errorCodes.find((c) => c === params.xato) ?? "telegram";
  const error = typeof params.xato === "string" ? t(`errors.${code}`) : null;

  const me = await getCurrentUser();
  if (me) redirect(me.onboarded ? next : "/boshlash");

  const startUrl = `${publicApiUrl}/v1/auth/telegram/start?return_to=${encodeURIComponent(next)}`;

  return (
    <Page className="grid min-h-[calc(var(--app-height)-4rem)] place-items-center py-14">
      <Reveal className="w-full max-w-sm text-center">
        <LogoMark className="mx-auto h-14" />
        <h1 className="display mt-8 text-4xl">{t("title")}</h1>
        <p className="mt-3 leading-relaxed text-muted-foreground">
          {t("lead")}
        </p>

        {error && (
          <Alert variant="destructive" className="mt-6 text-left">
            <AlertTitle>{t("failed")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {/* A plain link: the browser follows the API redirect to Telegram. */}
        <a
          href={startUrl}
          className={buttonVariants({
            size: "lg",
            className: "mt-8 h-12 w-full rounded-full bg-telegram text-base text-white transition-transform hover:bg-telegram-hover active:scale-[0.98]",
          })}
        >
          <TelegramIcon className="size-5" /> {t("button")}
        </a>

        <ul className="mt-10 space-y-4 border-t pt-8 text-left text-sm text-muted-foreground">
          <li className="flex gap-3">
            <Zap strokeWidth={1.5} className="mt-0.5 size-[1.125rem] shrink-0 text-foreground" />
            <span>
              <span className="font-medium text-foreground">{t("botTitle")}</span> {t("botText")}
            </span>
          </li>
          <li className="flex gap-3">
            <Lock strokeWidth={1.5} className="mt-0.5 size-[1.125rem] shrink-0 text-foreground" />
            <span>
              <span className="font-medium text-foreground">{t("phoneTitle")}</span> {t("phoneText")}
            </span>
          </li>
        </ul>
      </Reveal>
    </Page>
  );
}
