import { ViewTransition } from "react";

import { cn } from "@/lib/utils";

/** Page wrapper: animates route changes with the browser View Transitions API. */
export function Page({ className, children }: { className?: string; children: React.ReactNode }) {
  return (
    <ViewTransition enter="page-enter" exit="page-exit" default="none">
      <div className={cn("mx-auto w-full max-w-7xl px-4 sm:px-6", className)}>{children}</div>
    </ViewTransition>
  );
}
