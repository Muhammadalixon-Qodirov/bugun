"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { Loader2, SearchX } from "lucide-react";
import { m } from "motion/react";
import { useTranslations } from "next-intl";

import { EmptyState } from "@/components/empty-state";
import { ListingCard } from "@/components/listing-card";
import { ease } from "@/components/motion";
import { Button } from "@/components/ui/button";
import { listListings } from "@/lib/api/gen/sdk.gen";
import type { ListListingsData, ListListingsResponse } from "@/lib/api/gen/types.gen";

type Query = NonNullable<ListListingsData["query"]>;

// Cards far off screen skip layout and paint until scrolled near; long feeds stay light on phones.
const offscreen = "[content-visibility:auto] [contain-intrinsic-size:auto_22rem]";

/** Server-rendered first page plus "load more" pages fetched in the browser. */
export function ListingFeed({ query, initial }: { query: Query; initial: ListListingsResponse }) {
  const t = useTranslations("market");
  const perPage = initial.per_page;
  const feed = useInfiniteQuery({
    queryKey: ["feed", query],
    initialPageParam: 1,
    initialData: { pages: [initial], pageParams: [1] },
    queryFn: async ({ pageParam }) => {
      const { data } = await listListings({ query: { ...query, page: pageParam, per_page: perPage }, throwOnError: true });
      return data;
    },
    getNextPageParam: (last) => (last.page * last.per_page < last.total ? last.page + 1 : undefined),
    staleTime: 30_000,
  });
  if (feed.data.pages[0].listings.length === 0) {
    return (
      <EmptyState
        icon={<SearchX strokeWidth={1.5} />}
        title={t("emptyTitle")}
        description={t("emptyText")}
      />
    );
  }

  return (
    <>
      <div className="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 xl:grid-cols-4">
        {feed.data.pages.map((page, pi) =>
          page.listings.map((l, i) =>
            // The first page arrives with the HTML and is shown as is; only
            // pages loaded with "Ko'proq" fade in, lightly staggered.
            pi === 0 ? (
              <ListingCard key={l.id} l={l} className={offscreen} />
            ) : (
              <m.div
                key={l.id}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.3, ease, delay: Math.min(i, 8) * 0.03 }}
              >
                <ListingCard l={l} className={offscreen} />
              </m.div>
            ),
          ),
        )}
      </div>
      {feed.hasNextPage && (
        <div className="mt-12 flex justify-center">
          <Button variant="outline" size="lg" className="h-11 rounded-full px-8" disabled={feed.isFetchingNextPage} onClick={() => feed.fetchNextPage()}>
            {feed.isFetchingNextPage && <Loader2 className="animate-spin" />}
            {t("loadMore")}
          </Button>
        </div>
      )}
    </>
  );
}
