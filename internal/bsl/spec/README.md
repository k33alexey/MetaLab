# BSL foundation decision

MetaLab implements its BSL frontend and runtime independently in Go. The
selected pipeline is:

```text
source → lossless tokens → AST → semantic model → bytecode → Go VM / WASM VM
```

The lexer and parser remain native Go code without a parser-generator runtime.
Source spans and trivia are retained for diagnostics, formatting, navigation
and debugging. The same versioned bytecode contract is used by the server and
browser runtimes.

Open-source projects are comparison points, not drop-in implementation code:

- OneScript is a mature reference for BSL semantics and a stack VM, but its
  C#/.NET runtime does not fit the single-Go-binary requirement.
- OneBase demonstrates the complete business-platform flow in Go, but its DSL
  is a subset of BSL and uses a tree-walking interpreter tightly coupled to the
  application runtime.
- `LazarenkoA/1c-language-parser` demonstrates a Go/yacc AST, but introduces
  CGO, has a different diagnostics model and is not used as a dependency.
- `1c-syntax/bsl-parser` is a useful coverage reference, but its grammar is
  LGPL-licensed and is not copied or generated into MetaLab.

The syntax catalog and corpus in this directory are the acceptance boundary
for the frontend. Later iterations must turn every corpus file into executable
conformance tests and expand the catalog when compatible BSL syntax evolves.
