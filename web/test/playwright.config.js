import { defineConfig, devices } from "@playwright/test";

const PORT = 8788;

export default defineConfig({
  testDir: ".",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["html", { open: "never" }], ["list"]] : "list",
  outputDir: "test-results",

  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "retain-on-failure",
  },

  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],

  // Serves web/ as-is. build-wasm.sh must have run first; the "assets are
  // built" check in demo.spec.js reports it clearly if not.
  webServer: {
    command: `python3 -m http.server ${PORT} -d web`,
    url: `http://127.0.0.1:${PORT}/index.html`,
    reuseExistingServer: !process.env.CI,
    cwd: "../..",
  },
});
