"use client";

import { Share2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";

/** Native share sheet on phones, copy-to-clipboard elsewhere. */
export function ShareButton({ title }: { title: string }) {
  const t = useTranslations("contact");
  const share = async () => {
    const url = window.location.href;
    if (navigator.share) {
      try {
        await navigator.share({ title, url });
      } catch {
        // The user closed the share sheet.
      }
      return;
    }
    await navigator.clipboard.writeText(url);
    toast.success(t("linkCopied"));
  };
  return (
    <Button type="button" variant="outline" size="icon" className="shrink-0 rounded-full" onClick={share} aria-label={t("share")}>
      <Share2 />
    </Button>
  );
}
