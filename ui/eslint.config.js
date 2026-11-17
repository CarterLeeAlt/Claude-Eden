import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: [".generated/**", "vendor/**"] },
  {
    files: ["src/**/*.{ts,tsx}"],
    languageOptions: {
      parser: tseslint.parser,
      globals: { ...globals.browser, ...globals.node },
    },
    plugins: { "@typescript-eslint": tseslint.plugin },
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/ban-ts-comment": "error",
      "no-unreachable": "error",
      "valid-typeof": "error",
    },
  },
  {
    files: ["src/__tests__/*.mjs"],
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: { "no-undef": "error", "no-unreachable": "error", "valid-typeof": "error" },
  },
);
