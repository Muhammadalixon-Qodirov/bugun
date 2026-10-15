"use client";

import { type ErrorProps, RouteError } from "@/components/route-error";

export default function Error(props: ErrorProps) {
  return <RouteError {...props} />;
}
