import {
  Banknote,
  Circle,
  CloudRain,
  Fuel,
  Landmark,
  type LucideIcon,
  PackageOpen,
  PartyPopper,
  Route,
  Ship,
  Snowflake,
  Sun,
  Wheat,
} from "lucide-react";

import type { EventBody } from "@/lib/api/gen/types.gen";

export type EventKind = EventBody["kind"];

/** Market event kinds in the order the form offers them, each with its icon. */
export const eventKinds: { kind: EventKind; icon: LucideIcon }[] = [
  { kind: "frost", icon: Snowflake },
  { kind: "heat", icon: Sun },
  { kind: "rain", icon: CloudRain },
  { kind: "harvest", icon: Wheat },
  { kind: "export", icon: Ship },
  { kind: "import", icon: PackageOpen },
  { kind: "fuel", icon: Fuel },
  { kind: "currency", icon: Banknote },
  { kind: "holiday", icon: PartyPopper },
  { kind: "roads", icon: Route },
  { kind: "policy", icon: Landmark },
  { kind: "other", icon: Circle },
];

export function isEventKind(kind: string): kind is EventKind {
  return eventKinds.some((k) => k.kind === kind);
}

export const eventKindIcons: Record<string, LucideIcon> = Object.fromEntries(eventKinds.map((k) => [k.kind, k.icon]));
