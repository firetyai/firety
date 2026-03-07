#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const binaryName = process.platform === "win32" ? "firety.exe" : "firety";
const binaryPath = path.join(__dirname, "dist", binaryName);

if (!existsSync(binaryPath)) {
  console.error(
    "firety binary is missing. Reinstall the package or check the postinstall logs."
  );
  process.exit(1);
}

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: "inherit"
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

if (typeof result.status === "number") {
  process.exit(result.status);
}

process.exit(1);
