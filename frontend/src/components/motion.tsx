"use client";

import { LazyMotion, MotionConfig } from "motion/react";

import { cn } from "@/lib/utils";

export const ease = [0.22, 1, 0.36, 1] as const;

// Animation features (~30 KB) load in their own chunk right after hydration
// rather than with every page's first bundle.
const loadFeatures = () => import("@/components/motion-features").then((m) => m.default);

/** Loads motion features once and honours the OS "reduce motion" setting. */
export function MotionProvider({ children }: { children: React.ReactNode }) {
  return (
    <LazyMotion features={loadFeatures} strict>
      <MotionConfig reducedMotion="user" transition={{ duration: 0.45, ease }}>
        {children}
      </MotionConfig>
    </LazyMotion>
  );
}

type DivProps = React.HTMLAttributes<HTMLDivElement>;

/**
 * Fades content in with a CSS animation. The content is in the server HTML and
 * visible without JavaScript, so it never waits for hydration.
 */
export function Reveal({ delay = 0, className, style, ...props }: DivProps & { delay?: number }) {
  return <div className={cn("reveal", className)} style={{ ...style, animationDelay: delay ? `${delay}s` : undefined }} {...props} />;
}

/** Children fade in one after another (see .stagger in globals.css). */
export function Stagger({ className, ...props }: DivProps) {
  return <div className={cn("stagger", className)} {...props} />;
}

export function StaggerItem(props: DivProps) {
  return <div {...props} />;
}
