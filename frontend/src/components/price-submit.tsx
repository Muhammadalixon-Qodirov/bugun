"use client";

import { useMutation } from "@tanstack/react-query";
import { AlertTriangle, ArrowRight, Camera, Check, CornerDownLeft, LoaderCircle, Pencil, Sparkles } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useRef, useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { CategoryIcon } from "@/components/category-icon";
import { ease } from "@/components/motion";
import { useParserText } from "@/components/seller/parser-text";
import { PriceResult } from "@/components/seller/price-result";
import { VoiceInput } from "@/components/seller/voice-input";
import { Button } from "@/components/ui/button";
import {
  confirmPriceDraftMutation,
  createPriceDraftMutation,
  discardPriceDraftMutation,
  priceDraftFromPhotoMutation,
} from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Draft, Saved, Suggestion } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { priceExamples } from "@/lib/price-examples";

const examples = priceExamples;

const panel = {
  initial: { opacity: 0, y: 12, filter: "blur(4px)" },
  animate: { opacity: 1, y: 0, filter: "blur(0px)" },
  exit: { opacity: 0, y: -8, filter: "blur(4px)" },
  transition: { duration: 0.35, ease },
};

/** Which input helpers the API has turned on (GET /v1/ai/features). */
export type PriceInputFeatures = { voice: boolean; voiceMaxSeconds: number; photo: boolean };

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

export function PriceSubmit({ features }: { features: PriceInputFeatures }) {
  const router = useRouter();
  const t = useTranslations("price");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  const errors = useApiErrors();
  const parser = useParserText();
  const [text, setText] = useState("");
  const [draft, setDraft] = useState<Draft | null>(null);
  const [saved, setSaved] = useState<Saved[] | null>(null);
  const photoInput = useRef<HTMLInputElement>(null);

  const parse = useMutation({
    ...createPriceDraftMutation(),
    onSuccess: (d) => setDraft(d),
    onError: (err) => errors.show(err, "check"),
  });
  const confirm = useMutation({
    ...confirmPriceDraftMutation(),
    onSuccess: (res) => {
      setSaved(res.saved);
      setDraft(null);
      setText("");
      // Mahsulotlarim is rendered on the server: drop its cached copy.
      router.refresh();
    },
    onError: (err) => {
      errors.show(err, "save");
      setDraft(null);
    },
  });
  const discard = useMutation(discardPriceDraftMutation());
  const fromPhoto = useMutation({
    ...priceDraftFromPhotoMutation(),
    onSuccess: (res) => {
      // The text read from the photo goes into the box, so it can be fixed and sent again.
      setText(res.transcript);
      setDraft(res.draft);
    },
    onError: (err) => errors.show(err, "check"),
  });

  const edit = () => {
    if (draft?.id) discard.mutate({ path: { id: draft.id } });
    setDraft(null);
  };
  const submit = (value = text) => value.trim() && !parse.isPending && parse.mutate({ body: { text: value } });
  // A product word the parser was unsure of: send the text again with the chosen catalog name in its place.
  const choose = (word: string, name: string) => {
    if (draft?.id) discard.mutate({ path: { id: draft.id } });
    const next = text.replace(new RegExp(escapeRegExp(word), "iu"), name.toLocaleLowerCase("uz"));
    setText(next);
    setDraft(null);
    submit(next);
  };
  const addVoiceText = (spoken: string) => setText((prev) => (prev.trim() ? `${prev.trimEnd()}\n${spoken}` : spoken));
  const busy = parse.isPending || fromPhoto.isPending;
  const reviewing = draft !== null && draft.items.length > 0;

  return (
    <AnimatePresence mode="wait" initial={false}>
      {saved ? (
        <m.section key="result" {...panel}>
          <PriceResult saved={saved} onAgain={() => setSaved(null)} />
        </m.section>
      ) : reviewing ? (
        <m.section key="review" {...panel} className="overflow-hidden rounded-3xl border bg-card">
          <header className="flex items-center gap-2 border-b px-5 py-4">
            <Sparkles className="size-4 text-warm" />
            <h2 className="font-medium">{t("reviewTitle")}</h2>
            <span className="ml-auto text-xs text-muted-foreground">{t("reviewHint")}</span>
          </header>

          <ul className="divide-y">
            {draft.items.map((it, i) => (
              <m.li
                key={i}
                initial={{ opacity: 0, x: -8 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: 0.08 * i, duration: 0.35, ease }}
                className="flex gap-3 px-5 py-4"
              >
                <CategoryIcon category={it.category} />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline justify-between gap-x-3">
                    <span className="font-medium">{names.product(undefined, it.product_name)}</span>
                    <span className="num text-xl font-semibold">
                      {formatSom(it.price_som)}
                      <span className="ml-1 font-sans text-sm font-normal text-muted-foreground">{tc("perUnit", { unit: fmt.unit(it.unit) })}</span>
                    </span>
                  </div>
                  <div className="mt-1.5 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                    <span className="rounded-full bg-secondary px-2 py-0.5">
                      {it.trade_type === "ulgurji" ? tc("wholesaleLower") : it.trade_type === "chakana" ? tc("retailLower") : it.trade_type}
                    </span>
                    {it.quantity != null && (
                      <span className="rounded-full bg-secondary px-2 py-0.5">
                        {formatSom(it.quantity)} {fmt.unit(it.unit)}
                      </span>
                    )}
                  </div>
                  {parser.notes(it).map((n) => (
                    <p key={n} className="mt-2 flex items-center gap-1.5 text-xs text-warm-foreground">
                      <AlertTriangle className="size-3.5 text-warm" /> {n}
                    </p>
                  ))}
                </div>
              </m.li>
            ))}
          </ul>

          {draft.suggestions?.length ? (
            <div className="border-t px-5 py-3">
              <Suggestions suggestions={draft.suggestions} onChoose={choose} disabled={parse.isPending} />
            </div>
          ) : null}

          {draft.problems.length > 0 && (
            <div className="space-y-1 border-t bg-destructive/5 px-5 py-3 text-sm text-destructive">
              {draft.problems.map((p) => (
                <p key={p.raw}>{parser.problem(p)}</p>
              ))}
            </div>
          )}

          <footer className="flex flex-wrap gap-2 border-t bg-muted/30 px-5 py-4">
            <Button
              variant="solid"
              size="pill"
              className="h-11 flex-1 sm:flex-none sm:px-6"
              disabled={confirm.isPending}
              onClick={() => confirm.mutate({ path: { id: draft.id } })}
            >
              <Check /> {confirm.isPending ? tc("saving") : t("confirm")}
            </Button>
            <Button variant="outline" size="pill" className="h-11 flex-1 sm:flex-none" disabled={confirm.isPending} onClick={edit}>
              <Pencil /> {t("fix")}
            </Button>
          </footer>
        </m.section>
      ) : (
        <m.section key="compose" {...panel} className="rounded-3xl border bg-card p-2 transition-shadow focus-within:border-foreground/30 focus-within:shadow-[0_12px_40px_-16px_color-mix(in_oklch,var(--foreground)_25%,transparent)]">
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) submit();
            }}
            placeholder={examples.join("\n")}
            rows={5}
            maxLength={2000}
            autoFocus
            aria-label={t("textLabel")}
            className="w-full resize-none bg-transparent px-4 pt-3 text-lg outline-none placeholder:text-muted-foreground/60"
          />

          <AnimatePresence>
            {draft && draft.items.length === 0 && (
              <m.p
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                exit={{ opacity: 0, height: 0 }}
                className="mx-2 mb-2 rounded-xl bg-destructive/5 px-3 py-2 text-sm text-destructive"
              >
                {draft.problems.length > 0
                  ? draft.problems.map(parser.problem).join(" ")
                  : t("notRecognised")}
              </m.p>
            )}
          </AnimatePresence>
          {draft && draft.items.length === 0 && draft.suggestions?.length ? (
            <div className="mx-2 mb-2 px-1">
              <Suggestions suggestions={draft.suggestions} onChoose={choose} disabled={parse.isPending} />
            </div>
          ) : null}

          <div className="flex flex-wrap items-center gap-2 px-2 pb-1">
            <div className="flex flex-1 flex-wrap gap-1.5">
              {examples.map((ex) => (
                <button
                  key={ex}
                  type="button"
                  onClick={() => setText((prev) => (prev ? `${prev}\n${ex}` : ex))}
                  className="rounded-full border px-3 py-1 text-xs text-muted-foreground transition-colors hover:border-foreground/30 hover:text-foreground"
                >
                  + {ex}
                </button>
              ))}
            </div>
            {(features.voice || features.photo) && (
              <div className="flex w-full gap-2 sm:w-auto">
                {features.voice && features.voiceMaxSeconds > 0 && <VoiceInput maxSeconds={features.voiceMaxSeconds} onText={addVoiceText} disabled={busy} />}
                {features.photo && (
                  <>
                    <input
                      ref={photoInput}
                      type="file"
                      accept="image/jpeg,image/png,image/webp,image/heic"
                      className="sr-only"
                      tabIndex={-1}
                      aria-hidden
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        e.target.value = "";
                        if (file) fromPhoto.mutate({ body: { file } });
                      }}
                    />
                    <Button type="button" variant="outline" size="pill" className="h-11" disabled={busy} onClick={() => photoInput.current?.click()} data-testid="photo-button">
                      {fromPhoto.isPending ? <LoaderCircle className="animate-spin" /> : <Camera />}
                      {fromPhoto.isPending ? t("photo.reading") : t("photo.button")}
                    </Button>
                  </>
                )}
              </div>
            )}
            <Button variant="solid" size="pill" className="h-11 w-full sm:w-auto sm:px-6" disabled={!text.trim() || busy} onClick={() => submit()}>
              {parse.isPending ? t("checking") : t("check")}
              {parse.isPending ? null : <ArrowRight />}
            </Button>
          </div>
          <p className="hidden items-center gap-1 px-4 pt-1 pb-2 text-xs text-muted-foreground sm:flex">
            <CornerDownLeft className="size-3" /> {t("shortcut")}
          </p>
        </m.section>
      )}
    </AnimatePresence>
  );
}

/** Product words the parser was unsure of, each with catalog products to pick from. */
function Suggestions({ suggestions, onChoose, disabled }: { suggestions: Suggestion[]; onChoose: (word: string, name: string) => void; disabled?: boolean }) {
  const t = useTranslations("price");
  const names = useCatalogNames();
  return (
    <div className="space-y-2" data-testid="suggestions">
      {suggestions.map((s) => (
        <div key={s.word}>
          <p className="text-sm text-muted-foreground">{t("suggestions.title", { word: s.word })}</p>
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            {s.candidates.map((c) => (
              <button
                key={c.product_id}
                type="button"
                disabled={disabled}
                onClick={() => onChoose(s.word, c.name)}
                className="rounded-full border bg-background px-3 py-1 text-sm transition-colors hover:border-foreground/30 disabled:opacity-50"
              >
                {names.product(c.slug, c.name)}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
