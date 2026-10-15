import "server-only";

import { readFile } from "node:fs/promises";
import { join } from "node:path";

import { ImageResponse } from "next/og";

export const ogSize = { width: 1200, height: 630 };

// Theme tokens (globals.css) as plain colours: the image renderer has no CSS variables.
const colors = { background: "#fbfaf6", foreground: "#10231b", muted: "#55645d", accent: "#1c6e4d" };

const logo = readFile(join(process.cwd(), "public/logo-mark.png"), "base64").then((b) => `data:image/png;base64,${b}`);

/** A link-preview card in the site's colours: logo and brand line, a big title and a line under it. */
export async function ogCard({ eyebrow, title, text }: { eyebrow: string; title: string; text: string }) {
  const src = await logo;
  return new ImageResponse(
    (
      <div style={{ width: "100%", height: "100%", display: "flex", flexDirection: "column", justifyContent: "space-between", padding: 80, background: colors.background, color: colors.foreground }}>
        <div style={{ display: "flex", alignItems: "center", gap: 24 }}>
          {/* eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text */}
          <img src={src} width={60} height={71} />
          <span style={{ fontSize: 34, color: colors.muted }}>{eyebrow}</span>
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 28 }}>
          <span style={{ fontSize: 92, lineHeight: 1.05, letterSpacing: -2 }}>{title}</span>
          <span style={{ fontSize: 38, lineHeight: 1.35, color: colors.accent, maxWidth: 1000 }}>{text}</span>
        </div>
      </div>
    ),
    ogSize,
  );
}
