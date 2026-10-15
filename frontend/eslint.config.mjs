import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Generated from the backend OpenAPI spec.
    "src/lib/api/gen/**",
    // Playwright output.
    "playwright-report/**",
    "test-results/**",
  ]),
  {
    // One import path for the class-name helper (lib/utils re-exports the package).
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/lib/utils.ts"],
    rules: {
      "no-restricted-imports": ["error", { paths: [{ name: "cn", message: 'Import cn from "@/lib/utils".' }] }],
    },
  },
]);

export default eslintConfig;
