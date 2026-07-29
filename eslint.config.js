import js from "@eslint/js";
import globals from "globals";

export default [
  {
    // Vendored from GOROOT by web/build-wasm.sh -- never edited here.
    ignores: [
      "web/wasm_exec.js",
      "web/algo_dsp_demo.wasm",
      "node_modules/**",
      "web/test/playwright-report/**",
      "web/test/test-results/**",
    ],
  },
  {
    files: ["web/*.js"],
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: "script",
      globals: {
        ...globals.browser,
        // Loaded as classic scripts sharing globals, not ES modules. Until the
        // demo moves to `type="module"` these cross-file references are the
        // actual contract between the files.
        Go: "readonly",
        AlgoDSPDemo: "readonly",
        EQCanvas: "readonly",
        EffectChain: "readonly",
        DynamicsGraph: "readonly",
        DistChebGraph: "readonly",
      },
    },
    rules: {
      ...js.configs.recommended.rules,

      // Catching-and-ignoring is deliberate in the localStorage and theme
      // paths; requiring a named-but-unused binding there adds no safety.
      "no-unused-vars": [
        "error",
        { args: "after-used", caughtErrors: "none", varsIgnorePattern: "^_" },
      ],

      eqeqeq: ["error", "smart"],
      "no-var": "error",
      "prefer-const": "error",
      "no-implicit-globals": "off",
    },
  },
  {
    files: ["web/test/**/*.js", "eslint.config.js"],
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: "module",
      globals: {
        ...globals.node,
        // page.evaluate() callbacks are serialised and run in the browser, so
        // both browser globals and the demo's own script-level bindings are
        // legitimately in scope inside them.
        ...globals.browser,
        state: "readonly",
        readEffectsFromChain: "readonly",
      },
    },
    rules: js.configs.recommended.rules,
  },
];
