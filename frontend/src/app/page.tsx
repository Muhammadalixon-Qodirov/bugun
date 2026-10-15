import { MarketView } from "@/components/market-view";
import { Page } from "@/components/page";

export default async function HomePage({ searchParams }: PageProps<"/">) {
  return (
    <Page className="py-4 sm:py-6">
      <MarketView searchParams={await searchParams} basePath="/" />
    </Page>
  );
}
