import { ArrowRight, Camera, ChevronRight, Megaphone, Mic, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { EmptyState } from "@/components/empty-state";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { PriceSubmit } from "@/components/price-submit";
import { buttonVariants } from "@/components/ui/button";
import { priceExamples } from "@/lib/price-examples";
import { isEntrepreneur } from "@/lib/roles";
import { getAiFeatures } from "@/lib/server/ai";
import { getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("price");
  return { title: t("meta.title") };
}

// What each sample line in priceExamples is read as, in the same order.
const exampleKeys = ["tonnes", "perKilo", "wholesale"] as const;

export default async function PricePage() {
  const me = await getCurrentUser();
  if (!me) redirect("/kirish?keyin=/narx");
  if (!me.onboarded) redirect("/boshlash");
  const [t, ai] = await Promise.all([getTranslations("price"), getAiFeatures()]);

  if (!isEntrepreneur(me.role)) {
    return (
      <Page className="max-w-4xl py-10 sm:py-14">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <Reveal delay={0.1} className="mt-10">
          <EmptyState
            title={t("sellersOnly.title")}
            description={t("sellersOnly.description")}
            action={
              <Link href="/kabinet" className={buttonVariants({ variant: "outline", size: "pill" })}>
                {t("sellersOnly.action")}
              </Link>
            }
          />
        </Reveal>
      </Page>
    );
  }

  const voice = ai.voice && ai.voice_max_seconds > 0;

  return (
    <Page className="py-10 sm:py-14">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} description={t("description")} />
      <div className="mt-10 grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem] lg:gap-10 xl:gap-14">
        <Reveal delay={0.1} className="min-w-0">
          <PriceSubmit features={{ voice: ai.voice, voiceMaxSeconds: ai.voice_max_seconds, photo: ai.photo }} />
          <p className="mt-6 flex max-w-2xl items-start gap-2.5 px-1 text-sm leading-relaxed text-muted-foreground">
            <Megaphone className="mt-0.5 size-4 shrink-0" aria-hidden />
            {t("channelTip")}
          </p>
        </Reveal>

        <Reveal delay={0.15} className="space-y-4">
          <section className="rounded-3xl border bg-card p-5">
            <h2 className="font-semibold tracking-tight">{t("guide.title")}</h2>
            <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{t("guide.text")}</p>
            <ul className="mt-4 space-y-3">
              {priceExamples.map((ex, i) => (
                <li key={ex} className="text-sm">
                  <code className="block rounded-xl bg-muted px-3 py-2 font-mono text-[13px]">{ex}</code>
                  <span className="mt-1 flex items-start gap-1.5 px-1 text-xs leading-relaxed text-muted-foreground">
                    <ArrowRight className="mt-0.5 size-3 shrink-0" aria-hidden />
                    {t(`guide.examples.${exampleKeys[i]}`)}
                  </span>
                </li>
              ))}
            </ul>
            <p className="mt-4 text-xs leading-relaxed text-muted-foreground">{t("guide.lines")}</p>
          </section>

          {(voice || ai.photo) && (
            <section className="space-y-3 rounded-3xl border bg-card p-5 text-sm">
              {voice && (
                <p className="flex gap-3">
                  <Mic className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
                  <span className="leading-relaxed">{t("guide.voice", { seconds: ai.voice_max_seconds })}</span>
                </p>
              )}
              {ai.photo && (
                <p className="flex gap-3">
                  <Camera className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
                  <span className="leading-relaxed">{t("guide.photo")}</span>
                </p>
              )}
            </section>
          )}

          <Link href="/mahsulotlarim" className="group flex items-center gap-3 rounded-3xl border bg-card p-4 transition-colors hover:border-foreground/25">
            <span className="grid size-10 shrink-0 place-items-center rounded-2xl bg-muted">
              <Store strokeWidth={1.5} className="size-5" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block font-medium tracking-tight">{t("guide.mineTitle")}</span>
              <span className="block text-sm text-muted-foreground">{t("guide.mineText")}</span>
            </span>
            <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
          </Link>
        </Reveal>
      </div>
    </Page>
  );
}
