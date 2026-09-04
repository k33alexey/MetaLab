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
if (first.kind !== "number" || first.value !== 42) {
  throw new Error(`unexpected VM result: ${JSON.stringify(first)}`);
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
const invalidArgument = globalThis.MetaLabWasm.call(loaded.handle, "Add", [{}, 1]);
if (invalidArgument.ok) {
  throw new Error("unsupported JavaScript argument was accepted");
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
  writer.uint16(2);
  writer.uint32(4);

  writer.string("Add");
  writer.uint8(3); // function + export
  writer.uint16(2); // arity
  writer.uint16(2); // local count
  writer.uint16(2); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(4); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(1, 1); // load local 1
  writer.instruction(3, 0); // add
  writer.instruction(7, 0); // return

  writer.string("NotValue");
  writer.uint8(3);
  writer.uint16(1); // arity
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(3); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(9, 0); // not
  writer.instruction(7, 0); // return

  writer.string("Count");
  writer.uint8(3);
  writer.uint16(1); // arity
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(3); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(21, 0); // array length
  writer.instruction(7, 0); // return

  writer.string("EchoArray");
  writer.uint8(3);
  writer.uint16(1); // arity
  writer.uint16(1); // local count
  writer.uint16(1); // maximum stack depth
  writer.uint32(0); // constants
  writer.uint32(2); // instructions
  writer.instruction(1, 0); // load local 0
  writer.instruction(7, 0); // return
  return writer.bytes();
}
