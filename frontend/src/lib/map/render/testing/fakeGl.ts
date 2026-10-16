// A WebGL2 context for tests: it draws nothing and records every call.
// Objects it creates are named per kind in creation order (Buffer1,
// Texture3); uniform locations are named after their program and uniform
// (Program1.u_alpha). Typed array arguments are copied at call time, so a
// call shows the data as it was then.

export interface GlCall {
  name: string;
  args: unknown[];
  result: unknown;
}

export interface FakeGl {
  gl: WebGL2RenderingContext;
  calls: GlCall[];
  /** The test name of a created object or uniform location. */
  nameOf(object: unknown): string | undefined;
  /** A call as one line: name(args) -> result. Typed arrays show their size and a hash. */
  format(call: GlCall): string;
  /** The calls of one method. */
  callsOf(name: string): GlCall[];
}

function hashBytes(bytes: Uint8Array): string {
  let hash = 2166136261;
  for (let i = 0; i < bytes.length; i++) hash = Math.imul(hash ^ bytes[i], 16777619);
  return (hash >>> 0).toString(16);
}

/** onCall sees each call as it happens. */
export function fakeGl(onCall?: (call: GlCall) => void): FakeGl {
  const counts = new Map<string, number>();
  const names = new WeakMap<object, string>();
  const calls: GlCall[] = [];
  const named = (name: string) => {
    const object = {};
    names.set(object, name);
    return object;
  };
  const create = (kind: string) => {
    const count = (counts.get(kind) ?? 0) + 1;
    counts.set(kind, count);
    return named(`${kind}${count}`);
  };
  const formatValue = (value: unknown): string => {
    if (value === null || value === undefined) return String(value);
    if (ArrayBuffer.isView(value)) {
      const bytes = new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
      return `${value.constructor.name}[${bytes.length}]#${hashBytes(bytes)}`;
    }
    if (typeof value === "object") return names.get(value) ?? JSON.stringify(value);
    return String(value);
  };
  const copy = (value: unknown) => (ArrayBuffer.isView(value) && !(value instanceof DataView) ? (value as unknown as { slice(): unknown }).slice() : value);
  const gl = new Proxy({} as WebGL2RenderingContext, {
    get(_, name: string) {
      // Constants are their own names, so calls read like the source.
      if (/^[A-Z0-9_]+$/.test(name)) return name;
      return (...args: unknown[]) => {
        let result: unknown;
        if (name.startsWith("create")) result = create(name.slice("create".length));
        else if (name === "getUniformLocation") result = named(`${formatValue(args[0])}.${args[1]}`);
        else if (name === "getShaderParameter" || name === "getProgramParameter") result = true;
        else if (name === "getExtension") result = null;
        const call = { name, args: args.map(copy), result };
        calls.push(call);
        onCall?.(call);
        return result;
      };
    },
  });
  return {
    gl,
    calls,
    nameOf: (object) => (object && typeof object === "object" ? names.get(object) : undefined),
    format: (call) => `${call.name}(${call.args.map(formatValue).join(", ")})${call.result && typeof call.result === "object" ? ` -> ${formatValue(call.result)}` : ""}`,
    callsOf: (name) => calls.filter((call) => call.name === name),
  };
}
