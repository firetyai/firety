import fs from "node:fs";
import https from "node:https";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const packageJsonPath = path.join(__dirname, "..", "package.json");
const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, "utf8"));

const platform = mapPlatform(process.platform);
const arch = mapArch(process.arch);
const version = packageJson.version;

if (!platform || !arch) {
  console.warn(
    `firety npm install: unsupported platform ${process.platform}/${process.arch}; skipping binary download`
  );
  process.exit(0);
}

if (version.includes("development")) {
  console.warn(
    "firety npm install: development package version detected; skipping binary download"
  );
  process.exit(0);
}

const binaryName = process.platform === "win32" ? "firety.exe" : "firety";
const targetDir = path.join(__dirname, "dist");
const targetPath = path.join(targetDir, binaryName);
const assetName = `firety_${version}_${platform}_${arch}${process.platform === "win32" ? ".exe" : ""}`;
const url = `https://github.com/firetyai/firety/releases/download/v${version}/${assetName}`;

fs.mkdirSync(targetDir, { recursive: true });

await download(url, targetPath);

if (process.platform !== "win32") {
  fs.chmodSync(targetPath, 0o755);
}

function mapPlatform(value) {
  switch (value) {
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    case "win32":
      return "windows";
    default:
      return "";
  }
}

function mapArch(value) {
  switch (value) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      return "";
  }
}

function download(url, destination) {
  return new Promise((resolve, reject) => {
    const request = https.get(
      url,
      {
        headers: {
          "User-Agent": "firety-npm-installer"
        }
      },
      (response) => {
        if (
          response.statusCode &&
          response.statusCode >= 300 &&
          response.statusCode < 400 &&
          response.headers.location
        ) {
          response.resume();
          download(response.headers.location, destination).then(resolve, reject);
          return;
        }

        if (response.statusCode !== 200) {
          reject(
            new Error(
              `failed to download ${url}: unexpected status ${response.statusCode}`
            )
          );
          response.resume();
          return;
        }

        const file = fs.createWriteStream(destination);
        response.pipe(file);
        file.on("finish", () => {
          file.close(resolve);
        });
        file.on("error", (error) => {
          fs.rmSync(destination, { force: true });
          reject(error);
        });
      }
    );

    request.on("error", reject);
  });
}
