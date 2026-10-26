import { spawn } from "node:child_process";

export interface BrowserCommand {
  command: string;
  args: string[];
}

// The command that opens a URL in the default browser on each platform. The
// core CLI uses the same approach.
export function browserCommand(url: string, platform: NodeJS.Platform): BrowserCommand {
  if (platform === "darwin") return { command: "open", args: [url] };
  if (platform === "win32") return { command: "cmd", args: ["/c", "start", "", url] };
  return { command: "xdg-open", args: [url] };
}

// Opening the browser is a convenience, so a failure is reported to the caller
// and never fails the setup.
export function openBrowser(url: string, platform: NodeJS.Platform = process.platform): void {
  const { command, args } = browserCommand(url, platform);
  const child = spawn(command, args, { detached: true, stdio: "ignore" });
  child.on("error", () => undefined);
  child.unref();
}
