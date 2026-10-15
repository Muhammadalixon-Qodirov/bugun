import { Apple, Beef, BrickWall, Carrot, Milk, Package, Truck, Wheat, type LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

export const categoryIcons: Record<string, LucideIcon> = {
  sabzavot: Carrot,
  meva: Apple,
  gosht: Beef,
  don: Wheat,
  oziq_ovqat: Milk,
  qurilish: BrickWall,
  xizmat: Truck,
};

export function CategoryIcon({ category, className }: { category: string; className?: string }) {
  const Icon = categoryIcons[category] ?? Package;
  return (
    <span className={cn("grid size-10 shrink-0 place-items-center rounded-xl bg-muted text-foreground/80", className)}>
      <Icon strokeWidth={1.5} className="size-5" />
    </span>
  );
}
