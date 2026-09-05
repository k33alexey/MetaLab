import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";
import { performance } from "node:perf_hooks";

const [wasmPath, wasmExecPath] = process.argv.slice(2);
if (!wasmPath || !wasmExecPath) {
  throw new Error("usage: node wasm-smoke.mjs <module.wasm> <wasm_exec.js>");
}

await import(pathToFileURL(wasmExecPath));
const go = new globalThis.Go();
const wasm = await readFile(wasmPath);
const initializationStarted = performance.now();
const { instance } = await WebAssembly.instantiate(wasm, go.importObject);
void go.run(instance);
const initializationMilliseconds = performance.now() - initializationStarted;

if (!globalThis.MetaLabWasm) {
  throw new Error("MetaLabWasm API was not exported");
}

class BinaryWriter {
  constructor() {
    this.data = [];
  }

  uint8(value) {
    this.data.push(value & 0xff);
  }

  uint16(value) {
    this.uint8(value);
    this.uint8(value >>> 8);
  }

  uint32(value) {
    this.uint8(value);
    this.uint8(value >>> 8);
    this.uint8(value >>> 16);
    this.uint8(value >>> 24);
  }

  ascii(value) {
    for (const character of value) this.uint8(character.charCodeAt(0));
  }

  string(value) {
    const encoded = new TextEncoder().encode(value);
    this.uint32(encoded.byteLength);
    for (const byte of encoded) this.uint8(byte);
  }

  instruction(opcode, operand) {
    this.uint8(opcode);
    this.uint16(operand);
    for (let index = 0; index < 6; index += 1) this.uint32(0);
  }

  bytes() {
    return Uint8Array.from(this.data);
  }
}

const invalid = globalThis.MetaLabWasm.load(Uint8Array.of(0));
if (invalid.ok) {
  throw new Error("invalid bytecode was accepted");
}

const encoded = encodeAddProgram();
const loadStarted = performance.now();
const loaded = globalThis.MetaLabWasm.load(encoded);
const loadMicroseconds = (performance.now() - loadStarted) * 1_000;
assertSuccess(loaded, "load");

const first = globalThis.MetaLabWasm.call(loaded.handle, "add", [20, 22]);
assertSuccess(first, "call");
if (first.kind !== "number" || first.value !== 42 || first.text !== "42") {
  throw new Error(`unexpected VM result: ${JSON.stringify(first)}`);
}
const nested = globalThis.MetaLabWasm.call(loaded.handle, "Twice", [20, 22]);
assertSuccess(nested, "nested call");
if (nested.kind !== "number" || nested.value !== 84 || nested.text !== "84") {
  throw new Error(`unexpected nested VM result: ${JSON.stringify(nested)}`);
}
assertSuccess(globalThis.MetaLabWasm.call(loaded.handle, "SetState", [41]), "set module state");
const state = globalThis.MetaLabWasm.call(loaded.handle, "GetState", []);
assertSuccess(state, "get module state");
if (state.kind !== "number" || state.value !== 41 || state.text !== "41") {
  throw new Error(`unexpected module state: ${JSON.stringify(state)}`);
}
const failed = globalThis.MetaLabWasm.call(loaded.handle, "Fail", []);
if (failed.ok || failed.message !== "boom" || failed.stack?.length !== 1) {
  throw new Error(`unexpected runtime error: ${JSON.stringify(failed)}`);
}
if (failed.stack[0].module !== "Smoke" || failed.stack[0].function !== "Fail" || failed.stack[0].filename !== "smoke.bsl") {
  throw new Error(`unexpected runtime stack: ${JSON.stringify(failed.stack)}`);
}
const text = globalThis.MetaLabWasm.call(loaded.handle, "Add", ["Meta", "Lab"]);
assertSuccess(text, "string call");
if (text.kind !== "string" || text.value !== "MetaLab") {
  throw new Error(`unexpected string result: ${JSON.stringify(text)}`);
}
const boolean = globalThis.MetaLabWasm.call(loaded.handle, "NotValue", [false]);
assertSuccess(boolean, "boolean call");
if (boolean.kind !== "boolean" || boolean.value !== true) {
  throw new Error(`unexpected boolean result: ${JSON.stringify(boolean)}`);
}
const count = globalThis.MetaLabWasm.call(loaded.handle, "Count", [[1, 2, 3]]);
assertSuccess(count, "array call");
if (count.kind !== "number" || count.value !== 3) {
  throw new Error(`unexpected array length: ${JSON.stringify(count)}`);
}
const echoed = globalThis.MetaLabWasm.call(loaded.handle, "EchoArray", [[1, true, "three"]]);
assertSuccess(echoed, "array result");
if (echoed.kind !== "array" || JSON.stringify(echoed.value) !== '[1,true,"three"]') {
  throw new Error(`unexpected array result: ${JSON.stringify(echoed)}`);
}
const sourceDate = new Date(Date.UTC(2026, 8, 4, 15, 30, 45));
const echoedDate = globalThis.MetaLabWasm.call(loaded.handle, "EchoArray", [sourceDate]);
assertSuccess(echoedDate, "date result");
if (echoedDate.kind !== "date" || echoedDate.value.toISOString() !== sourceDate.toISOString()) {
  throw new Error(`unexpected date result: ${JSON.stringify(echoedDate)}`);
}
const echoedNull = globalThis.MetaLabWasm.call(loaded.handle, "EchoArray", [null]);
assertSuccess(echoedNull, "null result");
if (echoedNull.kind !== "null" || echoedNull.value !== null) {
  throw new Error(`unexpected null result: ${JSON.stringify(echoedNull)}`);
}
const exact = globalThis.MetaLabWasm.call(loaded.handle, "EchoArray", [{kind: "number", text: "0.1"}]);
assertSuccess(exact, "exact number result");
if (exact.kind !== "number" || exact.value !== 0.1 || exact.text !== "0.1") {
  throw new Error(`unexpected exact number result: ${JSON.stringify(exact)}`);
}
const invalidArgument = globalThis.MetaLabWasm.call(loaded.handle, "Add", [{}, 1]);
if (invalidArgument.ok) {
  throw new Error("unsupported JavaScript argument was accepted");
}
const serverOnly = globalThis.MetaLabWasm.call(loaded.handle, "ServerOnly", []);
if (serverOnly.ok || !serverOnly.error?.includes("server RPC is not configured")) {
  throw new Error(`server-only routine crossed the client boundary: ${JSON.stringify(serverOnly)}`);
}

const iterations = 20_000;
const started = performance.now();
for (let index = 0; index < iterations; index += 1) {
  const result = globalThis.MetaLabWasm.call(loaded.handle, "Add", [20, 22]);
  if (!result.ok || result.value !== 42) {
    throw new Error(`benchmark call failed at ${index}`);
  }
}
const elapsed = performance.now() - started;

const released = globalThis.MetaLabWasm.release(loaded.handle);
assertSuccess(released, "release");
const afterRelease = globalThis.MetaLabWasm.call(loaded.handle, "Add", [1, 2]);
if (afterRelease.ok) {
  throw new Error("released machine remained callable");
}

console.log(JSON.stringify({
  calls: iterations,
  nanosecondsPerCall: Math.round((elapsed * 1_000_000) / iterations),
  bytecodeBytes: encoded.byteLength,
  initializationMilliseconds: Number(initializationMilliseconds.toFixed(3)),
  loadMicroseconds: Math.round(loadMicroseconds),
}));
process.exit(0);

function assertSuccess(result, operation) {
  if (!result?.ok) {
    throw new Error(`${operation} failed: ${result?.error ?? "unknown error"}`);
  }
}

function encodeAddProgram() {
  const writer = new BinaryWriter();
  writer.ascii("MLBC");
  writer.uint16(6);
  writer.uint32(1); // modules
  writer.string("Smoke");
  writer.string("smoke.bsl");
  writer.uint32(1); // variables
  writer.string("State");
  writer.uint8(1); // exported
  writer.uint32(9);

  writer.string("Add");
  writer.uint16(0); // module
  writer.uint8(3); // function + export
  writer.uint8(0); // shared execution context
  writer.uint16(2); // arity
  writer.uint8(1); // parameter 0 by value
  writer.uint8(1); // parameter 1 by value
  writer.uint16(2); // local count
  writer.uint16(2); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(0); // module variable accesses
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(4); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(1, 1); // load local 1
  writer.instruction(3, 0); // add
  writer.instruction(7, 0); // return

  writer.string("NotValue");
  writer.uint16(0);
  writer.uint8(3);
  writer.uint8(0); // shared execution context
  writer.uint16(1); // arity
  writer.uint8(1); // parameter 0 by value
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(0); // module variable accesses
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(3); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(9, 0); // not
  writer.instruction(7, 0); // return

  writer.string("Count");
  writer.uint16(0);
  writer.uint8(3);
  writer.uint8(0); // shared execution context
  writer.uint16(1); // arity
  writer.uint8(1); // parameter 0 by value
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(0); // module variable accesses
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(3); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(21, 0); // array length
  writer.instruction(7, 0); // return

  writer.string("EchoArray");
  writer.uint16(0);
  writer.uint8(3);
  writer.uint8(0); // shared execution context
  writer.uint16(1); // arity
  writer.uint8(1); // parameter 0 by value
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(0); // module variable accesses
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(2); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(7, 0); // return

  writer.string("Twice");
  writer.uint16(0);
  writer.uint8(3);
  writer.uint8(0); // shared execution context
  writer.uint16(2); // arity
  writer.uint8(1); // parameter 0 by value
  writer.uint8(1); // parameter 1 by value
  writer.uint16(2); // local count
  writer.uint16(2); // maximum stack depth
  writer.uint32(1); // constants
  writer.uint8(1); // number
  writer.string("2");
  writer.uint32(0); // module variable accesses
  writer.uint32(1); // call sites
  writer.uint16(0); // Add target
  writer.uint8(0); // local call route
  writer.uint32(2); // argument references
  for (let index = 0; index < 2; index += 1) {
    writer.uint8(0); // no reference
    writer.uint16(0);
    writer.uint16(0);
  }
  writer.uint32(0); // exception handlers
  writer.uint32(6); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(1, 1); // load local 1
  writer.instruction(30, 0); // call site 0
  writer.instruction(0, 0); // constant 0
  writer.instruction(5, 0); // multiply
  writer.instruction(7, 0); // return

  writer.string("SetState");
  writer.uint16(0);
  writer.uint8(2); // procedure + export
  writer.uint8(0); // shared execution context
  writer.uint16(1); // arity
  writer.uint8(1); // parameter 0 by value
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(1); // constants
  writer.uint8(0); // undefined
  writer.uint32(1); // module variable accesses
  writer.uint8(2); // module reference
  writer.uint16(0); // module 0
  writer.uint16(0); // variable 0
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(4); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(29, 0); // store module access 0
  writer.instruction(0, 0); // undefined constant
  writer.instruction(7, 0); // return

  writer.string("GetState");
  writer.uint16(0);
  writer.uint8(3); // function + export
  writer.uint8(0); // shared execution context
  writer.uint16(0); // arity
  writer.uint16(0); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(1); // module variable accesses
  writer.uint8(2); // module reference
  writer.uint16(0); // module 0
  writer.uint16(0); // variable 0
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(2); // instructions
  writer.instruction(28, 0); // load module access 0
  writer.instruction(7, 0); // return

  writer.string("Fail");
  writer.uint16(0);
  writer.uint8(3); // function + export
  writer.uint8(0); // shared execution context
  writer.uint16(0); // arity
  writer.uint16(0); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(1); // constants
  writer.uint8(2); // string
  writer.string("boom");
  writer.uint32(0); // module variable accesses
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(4); // instructions
  writer.instruction(0, 0); // constant 0
  writer.instruction(31, 0); // raise
  writer.instruction(0, 0); // unreachable structural return value
  writer.instruction(7, 0); // return

  writer.string("ServerOnly");
  writer.uint16(0);
  writer.uint8(3); // function + export
  writer.uint8(2); // server execution context
  writer.uint16(0); // arity
  writer.uint16(0); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(1); // constants
  writer.uint8(1); // number
  writer.string("99");
  writer.uint32(0); // module variable accesses
  writer.uint32(0); // call sites
  writer.uint32(0); // exception handlers
  writer.uint32(2); // instructions
  writer.instruction(0, 0); // constant 0
  writer.instruction(7, 0); // return
  return writer.bytes();
}
