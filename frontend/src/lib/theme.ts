/** Page background per theme; also the browser bar and Telegram header colour. */
export const themeColors = { light: "#fbfaf6", dark: "#0c1511" } as const;

export type Scheme = keyof typeof themeColors;
