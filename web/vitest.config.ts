import { defineConfig } from "vitest/config";

// The Next.js app compiles with the automatic JSX runtime (tsconfig
// "jsx": "preserve"), so components do not import React. Match that in
// tests — otherwise rendering any shared component fails with
// "React is not defined" under the classic transform.
export default defineConfig({
  esbuild: { jsx: "automatic" },
});
