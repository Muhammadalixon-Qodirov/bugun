"use client";

import { ArrowUpRight, BellRing, CalendarRange, ChartSpline, Database, MapPinned, ShoppingBasket } from "lucide-react";
import { useTranslations } from "next-intl";

import { Chat } from "@/components/analyst/chat";

const skills = [
  { key: "prices", icon: ChartSpline },
  { key: "cheapest", icon: MapPinned },
  { key: "basket", icon: ShoppingBasket },
  { key: "forecast", icon: CalendarRange },
  { key: "actions", icon: BellRing },
] as const;

/**
 * The /tahlilchi conversation with a side column on wide screens: what the
 * assistant can answer, example questions that are asked on click, and where
 * the numbers come from.
 */
export function AssistantPage({ examples, initialQuestion, signedIn }: { examples: string[]; initialQuestion: string; signedIn: boolean }) {
  const t = useTranslations("analyst.page");
  return (
    <Chat
      examples={examples}
      initialQuestion={initialQuestion}
      signedIn={signedIn}
      aside={(ask, busy) => (
        <>
          <section className="rounded-3xl border bg-card p-5">
            <h2 className="font-semibold tracking-tight">{t("skillsTitle")}</h2>
            <ul className="mt-3 space-y-3">
              {skills.map(({ key, icon: Icon }) => (
                <li key={key} className="flex gap-3 text-sm leading-snug">
                  <span className="grid size-8 shrink-0 place-items-center rounded-xl bg-muted">
                    <Icon className="size-4" strokeWidth={1.75} aria-hidden />
                  </span>
                  <span className="min-w-0 pt-0.5">
                    <span className="block font-medium">{t(`skills.${key}.title`)}</span>
                    <span className="block text-muted-foreground">{t(`skills.${key}.text`)}</span>
                  </span>
                </li>
              ))}
            </ul>
          </section>

          <section className="rounded-3xl border bg-card p-5">
            <h2 className="font-semibold tracking-tight">{t("examplesTitle")}</h2>
            <div className="mt-3 grid gap-1.5">
              {examples.map((ex) => (
                <button
                  key={ex}
                  type="button"
                  disabled={busy}
                  onClick={() => ask(ex)}
                  className="group flex items-center gap-2 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-muted disabled:opacity-50"
                  data-testid="chat-aside-example"
                >
                  <span className="min-w-0 flex-1">{ex}</span>
                  <ArrowUpRight className="size-3.5 shrink-0 text-muted-foreground transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5" aria-hidden />
                </button>
              ))}
            </div>
          </section>

          <p className="flex gap-2.5 px-1 text-xs leading-relaxed text-muted-foreground">
            <Database className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            {t("dataNote")}
          </p>
        </>
      )}
    />
  );
}
