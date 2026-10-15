import { ChevronRight } from "lucide-react";
import Link from "next/link";

/** A titled, horizontally scrolling row of cards (edge-to-edge swipe on phones). */
export function Rail({
  title,
  href,
  hint,
  allLabel,
  children,
}: {
  title: string;
  href?: string;
  hint?: string;
  /** Text of the "see all" link, in the page's language. */
  allLabel?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="pt-14 sm:pt-20">
      <div className="mb-5 flex items-end justify-between gap-3">
        <div>
          <h2 className="display text-2xl sm:text-[1.75rem]">{title}</h2>
          {hint && <p className="mt-1.5 text-sm text-muted-foreground">{hint}</p>}
        </div>
        {href && allLabel && (
          <Link href={href} className="group flex shrink-0 items-center gap-0.5 text-sm font-medium underline-offset-4 hover:underline">
            {allLabel} <ChevronRight className="size-4 transition-transform group-hover:translate-x-0.5" />
          </Link>
        )}
      </div>
      <div className="-mx-4 flex snap-x snap-mandatory gap-4 overflow-x-auto px-4 pb-2 scrollbar-none sm:mx-0 sm:px-0 [&>*]:w-[44%] [&>*]:shrink-0 [&>*]:snap-start sm:[&>*]:w-[30%] md:[&>*]:w-[23%] lg:[&>*]:w-[18.8%]">
        {children}
      </div>
    </section>
  );
}
