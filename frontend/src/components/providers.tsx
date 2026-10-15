"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

import { MotionProvider } from "@/components/motion";
import { SmoothScroll } from "@/components/smooth-scroll";
import { TelegramApp } from "@/components/telegram-app";

export function Providers({ children, signedIn }: { children: React.ReactNode; signedIn: boolean }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { staleTime: 60_000, retry: 1 } },
      }),
  );
  return (
    <QueryClientProvider client={client}>
      <SmoothScroll />
      <TelegramApp signedIn={signedIn} />
      <MotionProvider>{children}</MotionProvider>
    </QueryClientProvider>
  );
}
