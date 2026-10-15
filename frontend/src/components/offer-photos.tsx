"use client";

import { ChevronLeft, ChevronRight, Package } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { categoryIcons } from "@/components/category-icon";
import { Photo } from "@/components/photo";

/** Swipeable photo strip for one offer. */
export function OfferPhotos({ photos, alt, category }: { photos: string[]; alt: string; category: string }) {
  const tc = useTranslations("common");
  const [i, setI] = useState(0);
  const Icon = categoryIcons[category] ?? Package;
  if (photos.length === 0) {
    return (
      <div className="grid size-full place-items-center bg-[radial-gradient(circle_at_50%_40%,var(--card),var(--muted)_75%)]">
        <Icon strokeWidth={1.25} className="size-10 text-muted-foreground/50" />
      </div>
    );
  }
  const go = (d: number) => setI((v) => (v + d + photos.length) % photos.length);
  return (
    <div className="group relative size-full overflow-hidden bg-muted">
      <AnimatePresence initial={false} mode="popLayout">
        <m.div
          key={photos[i]}
          className="absolute inset-0"
          initial={{ opacity: 0, scale: 1.03 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.3 }}
          drag={photos.length > 1 ? "x" : false}
          dragConstraints={{ left: 0, right: 0 }}
          onDragEnd={(_, info) => Math.abs(info.offset.x) > 40 && go(info.offset.x < 0 ? 1 : -1)}
        >
          <Photo thumb k={photos[i]} alt={alt} className="size-full" />
        </m.div>
      </AnimatePresence>
      {photos.length > 1 && (
        <>
          <button type="button" aria-label={tc("previous")} onClick={() => go(-1)} className="absolute top-1/2 left-1.5 hidden -translate-y-1/2 rounded-full bg-background/80 p-1 backdrop-blur group-hover:block">
            <ChevronLeft className="size-4" />
          </button>
          <button type="button" aria-label={tc("next")} onClick={() => go(1)} className="absolute top-1/2 right-1.5 hidden -translate-y-1/2 rounded-full bg-background/80 p-1 backdrop-blur group-hover:block">
            <ChevronRight className="size-4" />
          </button>
          <div className="absolute inset-x-0 bottom-2 flex justify-center gap-1">
            {photos.map((p, k) => (
              <span key={p} className={`h-1.5 rounded-full bg-white/90 shadow transition-all ${k === i ? "w-4" : "w-1.5 opacity-60"}`} />
            ))}
          </div>
        </>
      )}
    </div>
  );
}
