"use client";

import { useQuery } from "@tanstack/react-query";
import { createContext, use } from "react";

import { metaLimitsOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import { FALLBACK_LIMITS, type Limits, LIMITS_MAX_AGE_SECONDS } from "@/lib/limits";

const LimitsContext = createContext<Limits | null>(null);

/** Hands the limits the server already loaded (or null when it could not) to client components. */
export function LimitsProvider({ limits, children }: { limits: Limits | null; children: React.ReactNode }) {
  return <LimitsContext value={limits}>{children}</LimitsContext>;
}

/**
 * The API's business limits for forms and hints. Starts from the server's copy
 * (cached there for 5 minutes); when the server could not load them, the
 * browser asks once and uses the defaults meanwhile or when that fails too, so
 * a failed request never breaks a form.
 */
export function useLimits(): Limits {
  const initial = use(LimitsContext);
  const { data } = useQuery({
    ...metaLimitsOptions(),
    initialData: initial ?? undefined,
    staleTime: LIMITS_MAX_AGE_SECONDS * 1000,
    gcTime: Infinity,
    // One try per page view: a failed request is not repeated by every form that mounts.
    retry: false,
    retryOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  return data ?? FALLBACK_LIMITS;
}
