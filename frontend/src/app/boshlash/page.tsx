import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { ProfileForm } from "@/components/profile-form";
import { getCatalogData, getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth.onboarding");
  return { title: t("metaTitle") };
}

export default async function OnboardingPage() {
  const [me, catalog, t] = await Promise.all([getCurrentUser(), getCatalogData(), getTranslations("auth.onboarding")]);
  if (!me) redirect("/kirish?keyin=/boshlash");
  if (me.onboarded) redirect("/");

  return (
    <Page className="max-w-6xl py-10 sm:py-16">
      <Reveal>
        <ProfileForm
          wizard
          catalog={catalog}
          me={me}
          submitLabel={t("submit")}
          redirectTo="/"
          intro={
            <>
              <p className="eyebrow">{t("eyebrow")}</p>
              <h1 className="display mt-3 text-3xl sm:text-4xl">{t("hello", { name: me.display_name })}</h1>
              <p className="mt-3 leading-relaxed text-muted-foreground">
                {t("lead")}
              </p>
            </>
          }
        />
      </Reveal>
    </Page>
  );
}
