// @ts-nocheck
/**
 * OA Pi auth hot reload extension.
 *
 * Installed by `oa install pi` into ~/.pi/agent/extensions/.
 *
 * Pi keeps ~/.pi/agent/auth.json in memory for the lifetime of a session. This
 * extension reloads that in-memory auth state when `oa sync pi` rewrites the
 * auth file, so OpenAI Codex account rotation can take effect without
 * restarting Pi.
 */

import { getAgentDir, type ExtensionAPI, type ExtensionContext } from "@mariozechner/pi-coding-agent";
import { mkdirSync, statSync, watch } from "node:fs";
import { basename, join } from "node:path";

const STATUS_KEY = "oa-auth";
const AUTO_SYNC_COOLDOWN_MS = 30_000;
const AUTO_SYNC_TIMEOUT_MS = 120_000;

export default function (pi: ExtensionAPI) {
  const agentDir = getAgentDir();
  const authPath = join(agentDir, "auth.json");
  const authFile = basename(authPath);

  let stopWatch: (() => void) | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pendingReason: string | undefined;
  let lastMtime = getMtime();
  let lastAutoSyncAt = 0;
  let autoSyncInFlight = false;

  function getMtime(): number {
    try {
      return statSync(authPath).mtimeMs;
    } catch {
      return 0;
    }
  }

  function formatError(error: unknown): string {
    return error instanceof Error ? error.message : String(error);
  }

  function reloadAuth(ctx: ExtensionContext, reason: string): boolean {
    try {
      // Clear stale errors before this reload attempt, then surface only new ones.
      ctx.modelRegistry.authStorage.drainErrors();
      ctx.modelRegistry.authStorage.reload();

      const errors = ctx.modelRegistry.authStorage.drainErrors();
      if (errors.length > 0) {
        ctx.ui.setStatus(STATUS_KEY, "auth reload failed");
        ctx.ui.notify(`OA: auth.json reload failed: ${errors[0].message}`, "error");
        return false;
      }

      ctx.modelRegistry.refresh();
      lastMtime = getMtime();
      pendingReason = undefined;
      ctx.ui.setStatus(STATUS_KEY, `auth reloaded ${new Date().toLocaleTimeString()}`);
      ctx.ui.notify(`OA: Pi auth reloaded (${reason})`, "info");
      return true;
    } catch (error) {
      ctx.ui.setStatus(STATUS_KEY, "auth reload failed");
      ctx.ui.notify(`OA: Pi auth reload failed: ${formatError(error)}`, "error");
      return false;
    }
  }

  function scheduleReload(ctx: ExtensionContext, reason: string) {
    pendingReason = reason;

    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = undefined;

      if (!ctx.isIdle()) {
        ctx.ui.setStatus(STATUS_KEY, "auth reload pending");
        return;
      }

      reloadAuth(ctx, pendingReason ?? reason);
    }, 150);
  }

  async function syncAndReload(ctx: ExtensionContext, status: number) {
    if (ctx.model?.provider !== "openai-codex") return;
    if (status !== 401 && status !== 429) return;

    const now = Date.now();
    if (autoSyncInFlight || now - lastAutoSyncAt < AUTO_SYNC_COOLDOWN_MS) return;

    autoSyncInFlight = true;
    lastAutoSyncAt = now;
    ctx.ui.setStatus(STATUS_KEY, `oa sync pi --evenly after ${status}`);

    try {
      const result = await pi.exec("oa", ["sync", "pi", "--evenly"], {
        signal: ctx.signal,
        timeout: AUTO_SYNC_TIMEOUT_MS,
      });

      if (result.code !== 0) {
        const message = (result.stderr || result.stdout || `exit ${result.code}`).trim();
        ctx.ui.setStatus(STATUS_KEY, "oa sync pi failed");
        ctx.ui.notify(`OA: oa sync pi --evenly failed: ${message}`, "warning");
        return;
      }

      reloadAuth(ctx, `oa sync pi --evenly after HTTP ${status}`);
    } catch (error) {
      ctx.ui.setStatus(STATUS_KEY, "oa sync pi failed");
      ctx.ui.notify(`OA: oa sync pi --evenly failed: ${formatError(error)}`, "warning");
    } finally {
      autoSyncInFlight = false;
    }
  }

  pi.registerCommand("oa-auth-reload", {
    description: "Reload ~/.pi/agent/auth.json in memory without restarting Pi",
    handler: async (_args, ctx) => {
      await ctx.waitForIdle();
      reloadAuth(ctx, "manual command");
    },
  });

  pi.on("session_start", (_event, ctx) => {
    stopWatch?.();

    mkdirSync(agentDir, { recursive: true });
    lastMtime = getMtime();

    const watcher = watch(agentDir, { persistent: false }, (_eventType, filename) => {
      if (filename && filename.toString() !== authFile) return;

      const nextMtime = getMtime();
      if (nextMtime === lastMtime) return;

      scheduleReload(ctx, "auth.json changed");
    });

    watcher.unref?.();
    stopWatch = () => watcher.close();
  });

  // Safety net: if fs.watch misses an atomic replace, catch it before the next
  // provider call. This is the path that guarantees `oa sync pi` takes effect
  // on the next prompt even if the file watcher missed the event.
  pi.on("context", (_event, ctx) => {
    const nextMtime = getMtime();
    if (pendingReason || nextMtime !== lastMtime) {
      reloadAuth(ctx, pendingReason ?? "auth.json changed before provider request");
    }
  });

  pi.on("after_provider_response", async (event, ctx) => {
    await syncAndReload(ctx, event.status);
  });

  pi.on("agent_end", (_event, ctx) => {
    if (!pendingReason) return;
    reloadAuth(ctx, pendingReason);
  });

  pi.on("session_shutdown", () => {
    if (timer) clearTimeout(timer);
    timer = undefined;
    stopWatch?.();
    stopWatch = undefined;
  });
}
