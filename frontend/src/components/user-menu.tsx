"use client";

import { useMutation } from "@tanstack/react-query";
import { BellRing, Inbox, LayoutDashboard, LogOut, ShieldCheck, ShoppingBasket, Tag, UserRound } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { logoutMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import { isEntrepreneur } from "@/lib/roles";

function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
}

export function UserMenu({ name, onboarded, role, admin }: { name: string; onboarded: boolean; role?: string; admin: boolean }) {
  const entrepreneur = isEntrepreneur(role);
  const t = useTranslations("nav");
  const router = useRouter();
  const logout = useMutation({
    ...logoutMutation(),
    onSuccess: () => {
      router.push("/");
      router.refresh();
    },
  });

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="rounded-full outline-none ring-offset-2 ring-offset-background transition-shadow hover:ring-2 hover:ring-border focus-visible:ring-2 focus-visible:ring-ring" aria-label={t("profileMenu")}>
        <Avatar className="size-9">
          <AvatarFallback className="bg-accent text-xs font-medium text-accent-foreground">{initials(name)}</AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="truncate">{name}</DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        {entrepreneur && (
          <>
            <DropdownMenuItem onClick={() => router.push("/mahsulotlarim")}>
              <Tag /> {t("myListings")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => router.push("/kabinet")}>
              <Inbox /> {t("incoming")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => router.push("/sorovlar")}>
              <ShoppingBasket /> {t("requests")}
            </DropdownMenuItem>
          </>
        )}
        {/* Market administrations: the cabinet holds their reports. */}
        {onboarded && !entrepreneur && (
          <DropdownMenuItem onClick={() => router.push("/kabinet")}>
            <LayoutDashboard /> {t("cabinet")}
          </DropdownMenuItem>
        )}
        {onboarded && (
          <DropdownMenuItem onClick={() => router.push("/profil#kuzatuv")}>
            <BellRing /> {t("watches")}
          </DropdownMenuItem>
        )}
        {admin && (
          <DropdownMenuItem onClick={() => router.push("/admin")}>
            <ShieldCheck /> {t("admin")}
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => router.push(onboarded ? "/profil" : "/boshlash")}>
          <UserRound /> {onboarded ? t("profile") : t("finishSignUp")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => logout.mutate({})} disabled={logout.isPending}>
          <LogOut /> {t("signOut")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
