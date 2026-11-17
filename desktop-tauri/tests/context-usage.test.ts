import test from "node:test";
import assert from "node:assert/strict";
import { contextUsage } from "../src/domain/context-usage.ts";

test("no gauge until the provider reports a window and some usage", () => {
  assert.equal(contextUsage(undefined), undefined);
  assert.equal(contextUsage({ used: 1200, window: 0 }), undefined);
  assert.equal(contextUsage({ used: 0, window: 200000 }), undefined);
});

test("the percentage is rounded and clamped, with grouped counts", () => {
  assert.deepEqual(contextUsage({ used: 36000, window: 200000 }), { percent: 18, warning: false, label: "18% · 36,000 / 200,000" });
  assert.equal(contextUsage({ used: 1, window: 200000 })?.percent, 0);
  assert.equal(contextUsage({ used: 250000, window: 200000 })?.percent, 100);
});

test("above 80% the gauge warns and suggests /compact", () => {
  assert.equal(contextUsage({ used: 160000, window: 200000 })?.warning, false);
  assert.deepEqual(contextUsage({ used: 162000, window: 200000 }), { percent: 81, warning: true, label: "81% · 162,000 / 200,000 · Consider /compact" });
});
