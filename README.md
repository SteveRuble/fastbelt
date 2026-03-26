# Fastbelt

Fastbelt is a high-performance DSL toolkit for Go with a parser generator and Language Server Protocol (LSP) support.

It is designed for language tooling that needs low latency and good throughput on large workspaces.

For background and benchmarks, see the [Fastbelt introduction](https://www.typefox.io/blog/fastbelt-introduction/) blog post.

## Installation

Fastbelt ships as a Go module:

```sh
go get typefox.dev/fastbelt@latest
```

## Quick start

The first step is to write a grammar definition file, e.g. `grammar.fb`, which has a similar format as the grammar language of Langium.

To run the code generator for your grammar definition:

```sh
go run typefox.dev/fastbelt/cmd@latest -g ./grammar.fb -o ./
```

This writes generated Go files for services such as lexer, parser, linker, and type definitions.

## Examples

A minimal state machine example is available in `examples/statemachine`.

For editor integration, see the VS Code extension in `internal/vscode-extensions/statemachine`.

## Contributing

Issues and pull requests are welcome.

## License

Fastbelt is licensed under the [MIT License](./LICENSE).
