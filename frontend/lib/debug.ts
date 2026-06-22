// lib/debug.ts
// Structured debug logger — dev only, zero cost in production.
// Usage: const log = createLogger("signup")
//        log.step("calling backend")
//        log.ok("account created", { userId })
//        log.fail("signup failed", err)

const isDev = process.env.NODE_ENV === "development";

type LogLevel = "step" | "ok" | "fail" | "info";

const ICONS: Record<LogLevel, string> = {
  step: "→",
  ok: "✓",
  fail: "✗",
  info: "·",
};

const STYLES: Record<LogLevel, string> = {
  step: "color: #888; font-weight: normal",
  ok: "color: #22c55e; font-weight: bold",
  fail: "color: #ef4444; font-weight: bold",
  info: "color: #3b82f6; font-weight: normal",
};

function log(
  namespace: string,
  level: LogLevel,
  message: string,
  data?: unknown,
) {
  if (!isDev) return;

  const prefix = `[${namespace}] ${ICONS[level]}`;

  if (data !== undefined) {
    if (data instanceof Error) {
      console.groupCollapsed(`%c${prefix} ${message}`, STYLES[level]);
      console.error("message:", data.message);
      console.error("stack:", data.stack);
      console.groupEnd();
    } else {
      console.groupCollapsed(`%c${prefix} ${message}`, STYLES[level]);
      console.log(data);
      console.groupEnd();
    }
  } else {
    console.log(`%c${prefix} ${message}`, STYLES[level]);
  }
}

export function createLogger(namespace: string) {
  return {
    /** A step is starting */
    step: (message: string, data?: unknown) =>
      log(namespace, "step", message, data),
    /** Step succeeded */
    ok: (message: string, data?: unknown) =>
      log(namespace, "ok", message, data),
    /** Step failed */
    fail: (message: string, data?: unknown) =>
      log(namespace, "fail", message, data),
    /** General info */
    info: (message: string, data?: unknown) =>
      log(namespace, "info", message, data),
  };
}
