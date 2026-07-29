import { expect, test } from "@playwright/test";

const SETTINGS_KEY = "algo-dsp-settings";

/** Wait until the Go/WASM module has published its API and initialised. */
async function waitForDSP(page) {
  await page.waitForFunction(() => window.AlgoDSPDemo !== undefined, null, {
    timeout: 30_000,
  });
}

/** Collect console errors for the lifetime of a test. */
function trackConsoleErrors(page) {
  const errors = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") errors.push(msg.text());
  });
  page.on("pageerror", (err) => errors.push(String(err)));
  return errors;
}

test.describe("algo-dsp web demo", () => {
  test("loads, initialises WASM, and reports no console errors", async ({
    page,
  }) => {
    const errors = trackConsoleErrors(page);

    await page.goto("/");
    await waitForDSP(page);

    await expect(page.locator("#app-status")).toBeHidden();
    expect(errors).toEqual([]);
  });

  test("links back to the library it demonstrates", async ({ page }) => {
    await page.goto("/");

    const repo = page.locator('a[href="https://github.com/cwbudde/algo-dsp"]');
    await expect(repo.first()).toBeVisible();
    await expect(
      page.locator('a[href^="https://pkg.go.dev/"]').first(),
    ).toBeVisible();
  });

  test("renders non-silent audio when running", async ({ page }) => {
    await page.goto("/");
    await waitForDSP(page);

    const peak = await page.evaluate(() => {
      const api = window.AlgoDSPDemo;
      api.setRunning(true);
      let max = 0;
      for (let block = 0; block < 40; block += 1) {
        const out = api.render(1024);
        for (let i = 0; i < out.length; i += 1) {
          max = Math.max(max, Math.abs(out[i]));
        }
      }
      api.setRunning(false);
      return max;
    });

    expect(peak).toBeGreaterThan(0.001);
  });

  test("returns a plausible EQ response curve from Go", async ({ page }) => {
    await page.goto("/");
    await waitForDSP(page);

    const curve = await page.evaluate(() =>
      Array.from(
        window.AlgoDSPDemo.responseCurve(new Float32Array([100, 1000, 10000])),
      ),
    );

    expect(curve).toHaveLength(3);
    for (const db of curve) expect(Number.isFinite(db)).toBe(true);
    // Default chain is flat in the midband and rolled off at the extremes.
    expect(Math.abs(curve[1])).toBeLessThan(1);
    expect(curve[0]).toBeLessThan(curve[1]);
    expect(curve[2]).toBeLessThan(curve[1]);
  });

  test("an effect inserted into the chain reaches the Go engine", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForDSP(page);

    const result = await page.evaluate(() => {
      // `state` is a top-level `const` in app.js, which classic scripts share
      // through the global lexical environment.
      const chain = state.chain;
      const original = chain.getState();

      const input = original.nodes.find((n) => n.type === "_input");
      const output = original.nodes.find((n) => n.type === "_output");

      // Splice a chorus between the fixed input and output nodes.
      chain.setState({
        ...original,
        nodes: [
          ...original.nodes,
          {
            id: "test-chorus",
            type: "chorus",
            label: "Chorus",
            x: 420,
            y: 220,
            bypassed: false,
            fixed: false,
            params: { mix: 0.5, depth: 0.003, speedHz: 0.35, stages: 3 },
            pinnedParams: [],
          },
        ],
        connections: [
          { from: input.id, to: "test-chorus" },
          { from: "test-chorus", to: output.id },
        ],
      });

      const enabled = chain.getEnabledEffects().has("chorus");
      readEffectsFromChain();
      const err = window.AlgoDSPDemo.setEffects(state.effectsParams);
      const nodeCount = chain.getState().nodes.length;

      chain.setState(original);

      return {
        enabled,
        nodeCount,
        originalCount: original.nodes.length,
        err: err === null ? null : String(err),
        restored: chain.getState().nodes.length,
      };
    });

    expect(result.enabled).toBe(true);
    expect(result.nodeCount).toBe(result.originalCount + 1);
    expect(result.err).toBeNull();
    expect(result.restored).toBe(result.originalCount);
  });

  test("the analyser loop does not run while idle", async ({ page }) => {
    // It used to be started at page load and never stopped: 24 redraws a
    // second, forever, before audio was ever started and in hidden tabs, each
    // allocating several ~800-entry arrays and crossing into Go twice.
    await page.goto("/");
    await waitForDSP(page);

    const draws = await page.evaluate(
      () =>
        new Promise((resolve) => {
          const original = state.eqUI.draw.bind(state.eqUI);
          let count = 0;
          state.eqUI.draw = (...args) => {
            count += 1;
            return original(...args);
          };
          setTimeout(() => {
            state.eqUI.draw = original;
            resolve(count);
          }, 750);
        }),
    );

    expect(draws).toBe(0);
    expect(await page.evaluate(() => state.eqDrawLoopHandle)).toBeNull();
  });

  test("small plots size their backing store for the device pixel ratio", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForDSP(page);

    const result = await page.evaluate(() => {
      const canvas = document.getElementById("comp-graph");
      const cssWidth = state.compUI.cssWidth;

      Object.defineProperty(window, "devicePixelRatio", {
        value: 2,
        configurable: true,
      });
      state.compUI.draw();
      const backingAt2x = canvas.width;

      Object.defineProperty(window, "devicePixelRatio", {
        value: 1,
        configurable: true,
      });
      state.compUI.draw();

      return { cssWidth, backingAt2x, backingAt1x: canvas.width };
    });

    expect(result.backingAt2x).toBe(result.cssWidth * 2);
    expect(result.backingAt1x).toBe(result.cssWidth);
  });

  test("the step highlight follows the Go engine's clock", async ({ page }) => {
    // The highlight used to run off its own setInterval that re-derived step
    // timing from tempo/shuffle in JS, so it drifted away from the audio.
    await page.goto("/");
    await waitForDSP(page);

    await page.click("#run-toggle");

    const observed = await page.evaluate(
      () =>
        new Promise((resolve) => {
          const seen = new Set();
          const deadline = Date.now() + 2500;
          const poll = () => {
            const highlighted = document.querySelectorAll(".step.current");
            // Exactly one step is highlighted at a time.
            if (highlighted.length > 1) {
              resolve({ error: "multiple steps highlighted" });
              return;
            }
            seen.add(state.currentStep);
            if (seen.size >= 3 || Date.now() > deadline) {
              resolve({
                distinctSteps: seen.size,
                enginePosition: window.AlgoDSPDemo.currentStep(),
                mirrored: state.currentStep,
              });
              return;
            }
            requestAnimationFrame(poll);
          };
          poll();
        }),
    );

    await page.click("#run-toggle");

    expect(observed.error).toBeUndefined();
    expect(observed.distinctSteps).toBeGreaterThanOrEqual(3);
    // The mirror is at most one engine step behind.
    expect(Math.abs(observed.enginePosition - observed.mirrored)).toBeLessThan(
      16,
    );
  });

  test("the embedded impulse response library is available and audible", async ({
    page,
  }) => {
    // The IRs are compiled into the WASM binary via //go:embed, not fetched.
    // A duplicate, unreferenced copy used to sit at web/irs.irlib and was
    // removed; this pins the fact that the real library still works.
    await page.goto("/");
    await waitForDSP(page);

    const names = await page.evaluate(() =>
      Array.from(window.AlgoDSPDemo.getIRNames()),
    );
    expect(names.length).toBeGreaterThan(0);

    const energy = await page.evaluate((irName) => {
      const api = window.AlgoDSPDemo;
      const chain = state.chain;
      const original = chain.getState();
      const input = original.nodes.find((n) => n.type === "_input");
      const output = original.nodes.find((n) => n.type === "_output");

      // Energy remaining after the note has stopped: a convolution reverb
      // leaves a long tail where the dry signal leaves almost nothing.
      const tail = () => {
        api.setRunning(true);
        for (let i = 0; i < 30; i += 1) api.render(1024);
        api.setRunning(false);
        let sum = 0;
        for (let i = 0; i < 40; i += 1) {
          const out = api.render(1024);
          for (let j = 0; j < out.length; j += 1) sum += out[j] * out[j];
        }
        return sum;
      };

      const dry = tail();

      chain.setState({
        ...original,
        nodes: [
          ...original.nodes,
          {
            id: "ir",
            type: "reverb-conv",
            label: "Reverb (Conv IR)",
            x: 400,
            y: 220,
            bypassed: false,
            fixed: false,
            params: { irName, wet: 1.0, dry: 0.0 },
            pinnedParams: [],
          },
        ],
        connections: [
          { from: input.id, to: "ir" },
          { from: "ir", to: output.id },
        ],
      });
      readEffectsFromChain();
      const err = api.setEffects(state.effectsParams);

      const wet = tail();

      chain.setState(original);
      readEffectsFromChain();
      api.setEffects(state.effectsParams);

      return { dry, wet, err: err === null ? null : String(err) };
    }, names[0]);

    expect(energy.err).toBeNull();
    expect(energy.wet).toBeGreaterThan(energy.dry * 10);
  });

  test("survives corrupt persisted settings instead of bricking", async ({
    page,
  }) => {
    // Regression test. The Go accessors panic on a type mismatch, and an
    // unrecovered panic kills the WASM instance; because the bad value came
    // from localStorage, the reload replayed it and the demo stayed dead.
    await page.goto("/");
    await page.evaluate((key) => {
      localStorage.setItem(
        key,
        JSON.stringify({
          effectsParams: {
            chorusMix: null,
            reverbWet: "loud",
            unknownKey: 1,
          },
          compParams: { ratio: "four" },
          limParams: { threshold: {} },
        }),
      );
    }, SETTINGS_KEY);

    await page.reload();
    await waitForDSP(page);

    await expect(page.locator("#app-status")).toBeHidden();

    const alive = await page.evaluate(
      () => window.AlgoDSPDemo.render(128).length,
    );
    expect(alive).toBe(128);

    // The bad value must not have been adopted.
    const chorusMix = await page.inputValue("#chorus-mix");
    expect(Number.isFinite(Number(chorusMix))).toBe(true);
  });

  test("a bad parameter returns an error instead of killing the engine", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForDSP(page);

    const result = await page.evaluate(() => {
      const err = window.AlgoDSPDemo.setEffects({ chorusEnabled: "yes" });
      return {
        err: String(err),
        stillRenders: window.AlgoDSPDemo.render(64).length,
      };
    });

    expect(result.err).toContain("recovered");
    expect(result.stillRenders).toBe(64);
  });

  test("shows a visible message when the WASM module cannot load", async ({
    page,
  }) => {
    await page.route("**/algo_dsp_demo.wasm", (route) =>
      route.fulfill({ status: 404, body: "not found" }),
    );

    await page.goto("/");

    const status = page.locator("#app-status");
    await expect(status).toBeVisible();
    await expect(status).toHaveClass(/app-notice--error/);
    await expect(status).toContainText(/build-wasm|WebAssembly|could not be/i);
  });
});
