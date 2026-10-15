import { ListingGridSkeleton } from "@/components/listing-card";
import { Page } from "@/components/page";

export default function SellerLoading() {
  return (
    <Page className="py-5 sm:py-8">
      <div className="h-3.5 w-40 animate-pulse rounded bg-muted" />
      <div className="mt-5 h-64 animate-pulse rounded-3xl bg-muted sm:h-52" />
      <div className="mt-12 space-y-5 sm:mt-16">
        <div className="h-8 w-40 animate-pulse rounded-lg bg-muted" />
        <ListingGridSkeleton count={8} />
      </div>
    </Page>
  );
}
