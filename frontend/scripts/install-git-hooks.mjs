import { execFileSync, spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const frontendDir = dirname(fileURLToPath(import.meta.url));
const repository = spawnSync("git", ["rev-parse", "--show-toplevel"], {
  cwd: frontendDir,
  encoding: "utf8",
});
if (repository.status !== 0) process.exit(0);

const root = repository.stdout.trim();
const hooksDir = resolve(root, ".githooks");
const existing = spawnSync("git", ["config", "--local", "--get", "core.hooksPath"], {
  cwd: root,
  encoding: "utf8",
});

if (existing.status === 0 && resolve(root, existing.stdout.trim()) !== hooksDir) {
  console.warn(`Keeping existing Git hooks path: ${existing.stdout.trim()}`);
  process.exit(0);
}

execFileSync("git", ["config", "--local", "core.hooksPath", hooksDir], { cwd: root });
