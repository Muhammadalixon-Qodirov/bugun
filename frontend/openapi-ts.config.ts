import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../backend/openapi.json",
  output: { path: "src/lib/api/gen", postProcess: [] },
  plugins: [
    { name: "@hey-api/client-fetch", runtimeConfigPath: "./src/lib/api/client-config" },
    "@hey-api/typescript",
    "@hey-api/sdk",
    "@tanstack/react-query",
  ],
});
