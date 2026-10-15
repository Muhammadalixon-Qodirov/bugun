"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { createContext, use, useOptimistic, useTransition } from "react";

import { cn } from "@/lib/utils";

type QueryNavValue = { params: URLSearchParams; pending: boolean; set: (patch: Record<string, string | null>) => void };

const QueryNavContext = createContext<QueryNavValue | null>(null);

/**
 * Server-rendered pages whose filters live in the query string: a filter flips
 * at once (optimistic params) and the page dims until the new data arrives.
 */
export function QueryNav({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const [pending, startTransition] = useTransition();
  const [params, setParams] = useOptimistic(new URLSearchParams(search.toString()));
  const set = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(params);
    for (const [k, v] of Object.entries(patch)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    const qs = next.toString();
    startTransition(() => {
      setParams(next);
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    });
  };
  return <QueryNavContext value={{ params, pending, set }}>{children}</QueryNavContext>;
}

export function useQueryNav() {
  const value = use(QueryNavContext);
  if (!value) throw new Error("useQueryNav needs <QueryNav>");
  return value;
}

/** Content that fades while a filter change is loading. */
export function PendingArea({ className, children }: { className?: string; children: React.ReactNode }) {
  const { pending } = useQueryNav();
  return (
    <div aria-busy={pending} className={cn("transition-opacity duration-200", pending && "opacity-50", className)}>
      {children}
    </div>
  );
}
