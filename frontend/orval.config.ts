import { defineConfig } from "orval";

export default defineConfig({
  visto: {
    input: { target: "../api/openapi.yaml" },
    output: {
      mode: "single",
      target: "./src/generated/api.ts",
      schemas: "./src/generated/models",
      client: "react-query",
      httpClient: "fetch",
      override: {
        fetch: { includeHttpResponseReturnType: false, forceSuccessResponse: true },
        mutator: {
          path: "./src/lib/orvalMutator.ts",
          name: "customFetch",
        },
      },
    },
  },
});
