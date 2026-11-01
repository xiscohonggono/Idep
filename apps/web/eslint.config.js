import { nextJsConfig } from "@repo/eslint-config/next-js";

const noDirectSupabase = [
  {
    selector:
      "CallExpression[callee.type='MemberExpression'][callee.object.name=/supabase/i][callee.property.name=/^(from|rpc)$/]",
    message:
      "Guardrail violation (ADR-001): Do not query PostgreSQL directly from apps/web via Supabase (.from/.rpc). Route all domain reads and mutations through apps/api (Huma OpenAPI).",
  },
  {
    selector:
      "CallExpression[callee.type='MemberExpression'][callee.object.type='CallExpression'][callee.object.callee.name='createClient'][callee.property.name=/^(from|rpc)$/]",
    message:
      "Guardrail violation (ADR-001): Do not query PostgreSQL directly from apps/web via Supabase (.from/.rpc). Route all domain reads and mutations through apps/api (Huma OpenAPI).",
  },
];

const typeContractMessage =
  "Guardrail violation (Type Contracts): Do not hand-write DTO interfaces or object type literals in apps/web/lib/api/. Register Huma structs in apps/api, run pnpm codegen, and derive types from @/lib/api/schema.";

/** @type {import("eslint").Linter.Config[]} */
export default [
  ...nextJsConfig,
  {
    files: ["**/*.tsx"],
    languageOptions: {
      parserOptions: { babelOptions: { parserOpts: { plugins: ["jsx"] } } },
    },
  },
  {
    // The Babel parser records no TypeScript types in scope, so these misfire.
    files: ["**/*.{ts,tsx}"],
    rules: { "no-undef": "off", "no-unused-vars": "off" },
  },
  {
    // Paths are relative to apps/web. These globs also opt .ts/.tsx into ESLint.
    files: ["**/*.{js,jsx,ts,tsx}"],
    ignores: ["lib/api/schema.d.ts"],
    rules: {
      "no-restricted-syntax": ["error", ...noDirectSupabase],
    },
  },
  {
    files: ["lib/api/**/*.{ts,tsx}"],
    ignores: ["lib/api/schema.d.ts"],
    rules: {
      // Flat config replaces a rule's options, so this must repeat the Supabase ban.
      "no-restricted-syntax": [
        "error",
        ...noDirectSupabase,
        { selector: "TSInterfaceDeclaration", message: typeContractMessage },
        {
          selector: "TSTypeAliasDeclaration[typeAnnotation.type='TSTypeLiteral']",
          message: typeContractMessage,
        },
      ],
    },
  },
];
