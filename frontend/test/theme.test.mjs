import assert from "node:assert/strict";
import test from "node:test";
import { forcedColorScheme } from "../src/app/theme.ts";

test("Given the system theme, When resolving Mantine's override, Then OS preference remains active", () => {
  assert.equal(forcedColorScheme("system"), undefined);
});

test("Given an explicit theme, When resolving Mantine's override, Then the selected light or dark scheme is forced", () => {
  assert.equal(forcedColorScheme("light"), "light");
  assert.equal(forcedColorScheme("dark"), "dark");
});
