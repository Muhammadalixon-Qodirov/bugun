"use client";

import { Dialog } from "@base-ui/react/dialog";
import { ChevronLeft, ChevronRight, Maximize2, Package, X } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { categoryIcons } from "@/components/category-icon";
import { Photo } from "@/components/photo";
import { cn } from "@/lib/utils";

const ZOOM = 2.2;

type Origin = { x: number; y: number } | null;

/** Where the pointer is over an element, in percent, for transform-origin. */
function originAt(e: React.MouseEvent<HTMLElement>): Origin {
  const r = e.currentTarget.getBoundingClientRect();
  return { x: ((e.clientX - r.left) / r.width) * 100, y: ((e.clientY - r.top) / r.height) * 100 };
}

/**
 * Main photo with thumbnails. On desktop the photo magnifies under the pointer;
 * a click (or tap) opens a full-screen viewer with swipe, arrows and zoom.
 */
export function ListingGallery({ photos, alt, category }: { photos: string[]; alt: string; category: string }) {
  const t = useTranslations("media");
  const tc = useTranslations("common");
  const [i, setI] = useState(0);
  const [dir, setDir] = useState(1);
  const [lens, setLens] = useState<Origin>(null);
  const [open, setOpen] = useState(false);
  const Icon = categoryIcons[category] ?? Package;
  if (photos.length === 0) {
    return (
      <div className="grid aspect-square place-items-center rounded-3xl bg-[radial-gradient(circle_at_50%_40%,var(--card),var(--muted)_70%)]">
        <Icon strokeWidth={1} className="size-24 text-muted-foreground/40" />
      </div>
    );
  }
  const go = (d: number) => {
    setDir(d);
    setI((v) => (v + d + photos.length) % photos.length);
  };

  return (
    <div className="flex flex-col-reverse gap-3 sm:flex-row">
      {photos.length > 1 && (
        <div className="flex gap-2 sm:flex-col">
          {photos.map((p, k) => (
            <button
              key={p}
              type="button"
              onClick={() => {
                setDir(k > i ? 1 : -1);
                setI(k);
              }}
              aria-label={t("photoN", { n: k + 1 })}
              aria-current={k === i ? "true" : undefined}
              className={cn(
                "size-16 overflow-hidden rounded-xl border-2 transition-[border-color,opacity] sm:size-20",
                k === i ? "border-foreground" : "border-transparent opacity-60 hover:opacity-100",
              )}
            >
              <Photo thumb k={p} alt="" sizes="80px" className="size-full" />
            </button>
          ))}
        </div>
      )}
      <div className="group relative aspect-square flex-1 overflow-hidden rounded-3xl bg-muted">
        <AnimatePresence initial={false} custom={dir} mode="popLayout">
          <m.div
            key={photos[i]}
            custom={dir}
            className="absolute inset-0"
            initial={{ opacity: 0, x: dir * 60 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: dir * -60 }}
            transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
            drag={photos.length > 1 ? "x" : false}
            dragConstraints={{ left: 0, right: 0 }}
            onDragStart={() => setLens(null)}
            onDragEnd={(_, info) => Math.abs(info.offset.x) > 50 && go(info.offset.x < 0 ? 1 : -1)}
          >
            <button
              type="button"
              aria-label={t("enlarge")}
              onClick={() => setOpen(true)}
              onPointerMove={(e) => e.pointerType === "mouse" && setLens(originAt(e))}
              onPointerLeave={() => setLens(null)}
              className="size-full cursor-zoom-in overflow-hidden"
            >
              <Photo
                k={photos[i]}
                alt={alt}
                priority={i === 0}
                className="size-full transition-transform duration-200 ease-out"
                style={lens ? { transform: `scale(${ZOOM})`, transformOrigin: `${lens.x}% ${lens.y}%` } : undefined}
              />
            </button>
          </m.div>
        </AnimatePresence>
        <span className="pointer-events-none absolute bottom-3 left-3 flex items-center gap-1.5 rounded-full bg-black/50 px-2.5 py-1 text-xs font-medium text-white opacity-0 backdrop-blur-md transition-opacity group-hover:opacity-100">
          <Maximize2 className="size-3" /> {t("zoomIn")}
        </span>
        {photos.length > 1 && (
          <>
            <ArrowButton side="left" label={tc("previous")} onClick={() => go(-1)} className="opacity-0 group-hover:opacity-100" />
            <ArrowButton side="right" label={tc("next")} onClick={() => go(1)} className="opacity-0 group-hover:opacity-100" />
            <span className="num absolute right-3 bottom-3 rounded-full bg-black/50 px-2.5 py-1 text-xs font-medium text-white backdrop-blur-md">
              {i + 1} / {photos.length}
            </span>
          </>
        )}
      </div>

      <Viewer photos={photos} alt={alt} index={i} onIndex={(k) => setI(k)} open={open} onOpenChange={setOpen} />
    </div>
  );
}

function ArrowButton({ side, label, onClick, className }: { side: "left" | "right"; label: string; onClick: () => void; className?: string }) {
  const Icon = side === "left" ? ChevronLeft : ChevronRight;
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      className={cn(
        "absolute top-1/2 grid size-10 -translate-y-1/2 place-items-center rounded-full bg-background/85 shadow backdrop-blur transition-opacity",
        side === "left" ? "left-3" : "right-3",
        className,
      )}
    >
      <Icon className="size-5" />
    </button>
  );
}

/** Full-screen photo viewer: arrows or swipe to browse, click to zoom, Esc to close. */
function Viewer({
  photos,
  alt,
  index,
  onIndex,
  open,
  onOpenChange,
}: {
  photos: string[];
  alt: string;
  index: number;
  onIndex: (i: number) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("media");
  const tc = useTranslations("common");
  const [zoom, setZoom] = useState<Origin>(null);
  const go = (d: number) => {
    setZoom(null);
    onIndex((index + d + photos.length) % photos.length);
  };
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        setZoom(null);
        onOpenChange(o);
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-50 bg-black/95 transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
        <Dialog.Popup
          className="fixed inset-0 z-50 flex flex-col outline-none transition-opacity duration-200 data-[ending-style]:opacity-0 data-[starting-style]:opacity-0"
          onKeyDown={(e) => {
            if (photos.length < 2) return;
            if (e.key === "ArrowRight") go(1);
            if (e.key === "ArrowLeft") go(-1);
          }}
        >
          <Dialog.Title className="sr-only">{alt}</Dialog.Title>
          <div className="flex items-center justify-between px-4 pt-[calc(1rem+var(--app-safe-top))] pb-4 text-white">
            <span className="num text-sm text-white/70">{photos.length > 1 ? `${index + 1} / ${photos.length}` : ""}</span>
            <Dialog.Close aria-label={tc("close")} className="grid size-10 place-items-center rounded-full bg-white/10 transition-colors hover:bg-white/20">
              <X className="size-5" />
            </Dialog.Close>
          </div>
          <div className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden px-4 pb-[calc(1rem+var(--app-safe-bottom))]">
            <AnimatePresence initial={false} mode="popLayout">
              <m.div
                key={photos[index]}
                className="flex size-full items-center justify-center"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.2 }}
                drag={photos.length > 1 && !zoom ? "x" : false}
                dragConstraints={{ left: 0, right: 0 }}
                onDragEnd={(_, info) => Math.abs(info.offset.x) > 60 && go(info.offset.x < 0 ? 1 : -1)}
              >
                <button
                  type="button"
                  aria-label={zoom ? t("zoomOut") : t("zoomIn")}
                  onClick={(e) => setZoom(zoom ? null : originAt(e))}
                  onPointerMove={(e) => zoom && e.pointerType === "mouse" && setZoom(originAt(e))}
                  className={cn("size-full overflow-hidden", zoom ? "cursor-zoom-out" : "cursor-zoom-in")}
                >
                  <Photo
                    k={photos[index]}
                    alt={alt}
                    sizes="100vw"
                    className="size-full object-contain transition-transform duration-200 ease-out"
                    style={zoom ? { transform: `scale(${ZOOM})`, transformOrigin: `${zoom.x}% ${zoom.y}%` } : undefined}
                  />
                </button>
              </m.div>
            </AnimatePresence>
            {photos.length > 1 && (
              <>
                <ArrowButton side="left" label={tc("previous")} onClick={() => go(-1)} className="hidden sm:grid" />
                <ArrowButton side="right" label={tc("next")} onClick={() => go(1)} className="hidden sm:grid" />
              </>
            )}
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
