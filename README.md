# ext-parser-javascript

JavaScript parser plugin for **Onfly**. Built on [gotreesitter](https://github.com/odvcencio/gotreesitter) (pure-Go tree-sitter, no CGO) with the tree-sitter-javascript grammar. Compiles to a WASI module for Onfly's wazero sandbox, or a native binary for stdio use.

```bash
onfly ext install javascript
```

Handles `.js`, `.jsx`, `.mjs` and `.cjs` (JSX included). TypeScript is a separate plugin.

## Chunks

Input is one file on stdin, argv[1] is its path, output is the standard `[]Chunk` JSON.

| `type` | Source node | Notes |
|---|---|---|
| `import` | `import_statement` | `name` = module specifier |
| `export` | `export * from`, `export { a }`, `export default expr` | declarations are chunked as themselves |
| `class` | `class_declaration`, `export default class` | header only (JSDoc, decorators, signature); members are separate chunks |
| `method` `property` `static_block` | class body members | `receiver` = owning class; arrow-function fields are `method` |
| `function` | function declarations, and `const f = () => {}` / `function` expressions | |
| `constant` `variable` | top-level `const` / `let` and `var` | one chunk per declarator when a statement declares several |
| `statement` | other top-level code | consecutive statements merged (`module.exports = ...` lands here) |
| `file` | whole file | fallback when nothing else matched |

Metadata: `name`, `fqn`, `receiver`, `start_line`, `end_line`, and where they apply `signature`, `params`, `kind`, `visibility` (`#private`), `modifiers`, `decorators`, `extends`, `exported`, `default`, `part`.

JSDoc, `//` comments directly above a declaration, and decorators are part of the chunk. Bodies over 8 KB are split at line boundaries (`part=1/3`). Files over 80 KiB are not parsed: they come back as plain `file` chunks (`reason=too_large`), and files over 2 MiB yield `[]`. The host cancels a parse at 5 s (and then keeps the stale chunks). Syntax errors do not fail the file: what parsed is chunked and a warning goes to stderr.

## Known limitations

- The wasm is about 18 MB (tree-sitter runtime plus the JavaScript grammar). Expect a few seconds to compile it once per host start.
- CommonJS exports (`module.exports = ...`, `exports.x = ...`) are plain `statement` chunks, not `export`.
- Declarations inside blocks (`if`, `try`) and function bodies are part of their parent chunk, not separate chunks.
- Vue / Svelte templates are not parsed; only the `.js` files around them.

## Build

```bash
make wasm     # bin/js-parser.wasm (reproducible, Go 1.27+)
make native   # bin/js-parser
make test
```

## Use

```yaml
indexer:
  include: ["**/*.js", "**/*.jsx"]
  parsers:
    - extensions: [".js", ".jsx", ".mjs", ".cjs"]
      type: "wasm"
      path: ".onfly/plugins/js-parser.wasm"
```

Third-party licenses: see THIRD_PARTY_NOTICES.
