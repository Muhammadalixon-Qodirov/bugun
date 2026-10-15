import { cn } from "@/lib/utils";

/** The "B" mark: the letter with a magnifier over a growth chart. */
export function LogoMark({ className }: { className?: string }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img src="/logo-mark.png" alt="" width={298} height={353} className={cn("h-7 w-auto dark:invert", className)} />
  );
}

/** Wordmark: the mark stands in for the "B" of "Bugun". */
export function Logo({ className, compact }: { className?: string; compact?: boolean }) {
  return (
    <span role="img" aria-label="Bugun" className={cn("flex items-end text-[1.45rem] leading-none font-bold tracking-tight", className)}>
      <LogoMark className="-mr-px h-[1.72rem]" />
      <span aria-hidden className={cn("pb-px", compact && "hidden sm:inline")}>
        ugun
      </span>
    </span>
  );
}
