import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { AssistantPage as AssistantWorkspace } from "@/components/analyst/assistant-page";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("analyst");
  return { title: t("meta.title"), description: t("meta.description") };
}

const exampleKeys = ["price", "basket", "forecast", "movers", "cheapest", "help"] as const;

export default async function AssistantPage({ searchParams }: PageProps<"/tahlilchi">) {
  const [{ q }, me, t] = await Promise.all([searchParams, getCurrentUser(), getTranslations("analyst")]);

  return (
    <Page className="py-10 sm:py-14">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} description={t("description")} />
      <div className="mt-8">
        <AssistantWorkspace
          examples={exampleKeys.map((k) => t(`examples.${k}`))}
          initialQuestion={typeof q === "string" ? q.slice(0, 500) : ""}
          signedIn={!!me}
        />
      </div>
    </Page>
  );
}
