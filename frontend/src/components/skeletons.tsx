import { ListingGridSkeleton } from "@/components/listing-card";
import { Page } from "@/components/page";

/** Shown instantly while a market page renders on the server. */
export function MarketSkeleton() {
  return (
    <Page className="py-4 sm:py-6">
      <div className="flex gap-9 overflow-hidden border-b pb-3 lg:justify-between">
        {Array.from({ length: 9 }, (_, i) => (
          <div key={i} className="flex shrink-0 flex-col items-center gap-2 pt-2">
            <div className="size-6 animate-pulse rounded-md bg-muted" />
            <div className="h-3 w-14 animate-pulse rounded bg-muted" />
          </div>
        ))}
      </div>
      <div className="mt-8 grid gap-8 lg:grid-cols-[15rem_1fr]">
        <div className="hidden space-y-4 lg:block">
          <div className="h-5 w-20 animate-pulse rounded bg-muted" />
          <div className="h-28 animate-pulse rounded-2xl bg-muted" />
          <div className="h-28 animate-pulse rounded-2xl bg-muted" />
        </div>
        <div className="space-y-6">
          <div className="flex items-end justify-between gap-4">
            <div className="space-y-2">
              <div className="h-8 w-56 animate-pulse rounded-lg bg-muted" />
              <div className="h-4 w-40 animate-pulse rounded bg-muted" />
            </div>
            <div className="h-10 w-48 animate-pulse rounded-full bg-muted" />
          </div>
          <ListingGridSkeleton count={12} />
        </div>
      </div>
    </Page>
  );
}

export function DetailSkeleton() {
  return (
    <Page className="py-5 sm:py-8">
      <div className="h-3.5 w-52 animate-pulse rounded bg-muted" />
      <div className="mt-5 grid gap-8 lg:grid-cols-[1.2fr_1fr] lg:gap-14">
        <div className="aspect-square animate-pulse rounded-3xl bg-muted" />
        <div className="space-y-4">
          <div className="h-3 w-32 animate-pulse rounded bg-muted" />
          <div className="h-10 w-3/4 animate-pulse rounded-lg bg-muted" />
          <div className="h-4 w-1/2 animate-pulse rounded bg-muted" />
          <div className="mt-7 h-72 animate-pulse rounded-3xl bg-muted" />
          <div className="h-48 animate-pulse rounded-3xl bg-muted" />
        </div>
      </div>
    </Page>
  );
}

function HeaderSkeleton() {
  return (
    <div className="space-y-3">
      <div className="h-3 w-24 animate-pulse rounded bg-muted" />
      <div className="h-10 w-full max-w-md animate-pulse rounded-lg bg-muted" />
      <div className="h-4 w-full max-w-xl animate-pulse rounded bg-muted" />
    </div>
  );
}

/** Statistics, map, comparison, province and product analysis pages. */
export function ChartsSkeleton() {
  return (
    <Page className="py-10 sm:py-14">
      <HeaderSkeleton />
      <div className="mt-8 flex gap-2 overflow-hidden">
        {Array.from({ length: 5 }, (_, i) => (
          <div key={i} className="h-9 w-24 shrink-0 animate-pulse rounded-full bg-muted" />
        ))}
      </div>
      <div className="mt-6 h-[min(28rem,60vh)] animate-pulse rounded-3xl bg-muted" />
      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <div className="h-64 animate-pulse rounded-3xl bg-muted" />
        <div className="h-64 animate-pulse rounded-3xl bg-muted" />
      </div>
    </Page>
  );
}

/** Signed-in pages: the cabinet, own listings, requests, profile and admin. */
export function DashboardSkeleton() {
  return (
    <Page className="max-w-6xl py-10 sm:py-14">
      <HeaderSkeleton />
      <div className="mt-10 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <div key={i} className="h-24 animate-pulse rounded-2xl bg-muted" />
        ))}
      </div>
      <div className="mt-8 space-y-3">
        {Array.from({ length: 5 }, (_, i) => (
          <div key={i} className="h-20 animate-pulse rounded-2xl bg-muted" />
        ))}
      </div>
    </Page>
  );
}

/** Single-form pages such as price submission. */
export function FormSkeleton() {
  return (
    <Page className="max-w-2xl py-10 sm:py-14">
      <HeaderSkeleton />
      <div className="mt-10 h-96 animate-pulse rounded-3xl bg-muted" />
    </Page>
  );
}
