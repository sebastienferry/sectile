import assert from "node:assert/strict";
import test from "node:test";
import { epicColorHex } from "../src/domain/epic-color.ts";

test("epic colors match the existing Sectile palette and hash", () => {
  assert.equal(epicColorHex("PE-1078"), "#f59e0b");
  assert.equal(epicColorHex("EPIC-1"), "#10f070");
  assert.equal(epicColorHex(" FIN-505 "), "#ffd000");
  assert.equal(epicColorHex(""), undefined);
});
