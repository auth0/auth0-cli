import { ApiError } from "./api.js";
import { UsageError } from "./args.js";
import type { IoStreams } from "./cli.js";
import { PluginError } from "./plugin-error.js";
import type { Ui } from "./ui.js";

export function reportError(error: unknown, json: boolean, io: IoStreams, ui: Ui): number {
  const { code, message, details } = describeError(error);
  if (json) {
    io.stderr.write(`${JSON.stringify({ error: code, message, ...details })}\n`);
    return 1;
  }
  ui.error(message);
  for (const fix of details.fixes ?? []) ui.detail(fix);
  return error instanceof UsageError ? 2 : 1;
}

export function describeError(error: unknown): { code: string; message: string; details: { fixes?: string[]; status?: number } } {
  if (error instanceof PluginError) {
    return { code: error.code, message: error.message, details: error.fixes ? { fixes: error.fixes } : {} };
  }
  if (error instanceof ApiError) {
    return { code: error.errorCode ?? "api_error", message: error.message, details: { status: error.status } };
  }
  if (error instanceof Error) return { code: "setup_failed", message: error.message, details: {} };
  return { code: "setup_failed", message: String(error), details: {} };
}
