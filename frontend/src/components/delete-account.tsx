"use client";

import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { deleteMeMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useApiErrors } from "@/lib/errors";

export function DeleteAccount() {
  const router = useRouter();
  const t = useTranslations("profile.delete");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const [confirming, setConfirming] = useState(false);
  const remove = useMutation({
    ...deleteMeMutation(),
    onSuccess: () => {
      toast.success(t("deleted"));
      router.push("/");
      router.refresh();
    },
    onError: (err) => errors.show(err, "delete"),
  });

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
      <p className="flex-1 text-sm leading-relaxed text-muted-foreground">
        {confirming
          ? t("confirm")
          : t("warning")}
      </p>
      <div className="flex gap-2">
        {confirming ? (
          <>
            <Button variant="destructive" size="pill" disabled={remove.isPending} onClick={() => remove.mutate({})}>
              {t("yes")}
            </Button>
            <Button variant="ghost" size="pill" onClick={() => setConfirming(false)}>
              {tc("cancel")}
            </Button>
          </>
        ) : (
          <Button variant="outline" size="pill" className="text-destructive hover:text-destructive" onClick={() => setConfirming(true)}>
            {t("button")}
          </Button>
        )}
      </div>
    </div>
  );
}
