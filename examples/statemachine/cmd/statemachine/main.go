// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Command statemachine is a minimal stdio-oriented example: load a .statemachine file through the
// same workspace and document pipeline used in production, report diagnostics, print a short AST
// summary, then optionally step the machine by reading event names from stdin.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/examples/statemachine"
	"typefox.dev/fastbelt/textdoc"
	"typefox.dev/fastbelt/workspace"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	var path string
	switch len(args) {
	case 0:
		path = "-"
	case 1:
		path = args[0]
	default:
		return fmt.Errorf("usage: %s [path.statemachine|-]", filepath.Base(os.Args[0]))
	}

	content, sourceLabel, fromStdin, err := readSource(path)
	if err != nil {
		return err
	}

	var docURI core.URI
	if sourceLabel == "<stdin>" {
		docURI = core.FileURI("/stdin.statemachine")
	} else {
		docURI = core.FileURI(sourceLabel)
	}
	fileDoc, ferr := textdoc.NewFile(docURI.DocumentURI(), "statemachine", 0, string(content))
	if ferr != nil {
		return ferr
	}
	document := core.NewDocument(fileDoc)

	srv := statemachine.CreateServices()
	srv.Workspace().DocumentManager.Set(document)

	ctx := context.Background()
	var buildErr error
	srv.Workspace().Lock.Write(ctx, func(buildCtx context.Context, downgrade func()) {
		buildErr = srv.Workspace().Builder.Build(buildCtx, []*core.Document{document}, downgrade)
	})
	if buildErr != nil {
		return fmt.Errorf("build: %w", buildErr)
	}

	printDiagnostics(os.Stderr, document)
	if fatalBuildIssues(document) {
		return fmt.Errorf("document has errors (see diagnostics above)")
	}

	if document.Root == nil {
		return fmt.Errorf("no AST root after build")
	}
	sm, ok := document.Root.(statemachine.Statemachine)
	if !ok {
		return fmt.Errorf("expected a statemachine root AST, got %T", document.Root)
	}

	printSummary(os.Stdout, sm)

	if fromStdin {
		fmt.Fprintf(os.Stderr, "\n(source was stdin; not reading events from stdin)\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "\nEnter event names, one per line (EOF to stop). Unknown events are reported.\n")
	return simulate(os.Stdout, os.Stdin, sm)
}

func readSource(path string) (content []byte, label string, fromStdin bool, err error) {
	if path == "-" {
		b, rerr := io.ReadAll(os.Stdin)
		return b, "<stdin>", true, rerr
	}
	abs, rerr := filepath.Abs(path)
	if rerr != nil {
		return nil, "", false, rerr
	}
	b, rerr := os.ReadFile(abs)
	return b, abs, false, rerr
}

func printDiagnostics(w io.Writer, doc *core.Document) {
	for _, d := range workspace.CreateLexerDiagnostics(doc) {
		fmt.Fprintf(w, "%s\n", formatDiagnostic("lexer", &d))
	}
	for _, d := range workspace.CreateParserDiagnostics(doc) {
		fmt.Fprintf(w, "%s\n", formatDiagnostic("parser", &d))
	}
	for _, d := range workspace.CreateLinkerDiagnostics(doc) {
		fmt.Fprintf(w, "%s\n", formatDiagnostic("linker", &d))
	}
	for _, d := range doc.Diagnostics {
		fmt.Fprintf(w, "%s\n", formatDiagnostic("validate", d))
	}
}

func formatDiagnostic(source string, d *core.Diagnostic) string {
	loc := formatLocation(d.Range.Start)
	return fmt.Sprintf("%s %s: %s", source, loc, d.Message)
}

func formatLocation(l core.TextLocation) string {
	// Text locations follow LSP conventions (0-based line and column).
	return fmt.Sprintf("line %d col %d", int(l.Line)+1, int(l.Column)+1)
}

func fatalBuildIssues(doc *core.Document) bool {
	if len(doc.LexerErrors) > 0 || len(doc.ParserErrors) > 0 {
		return true
	}
	for _, ref := range doc.References {
		if ref.Error() != nil {
			return true
		}
	}
	for _, d := range doc.Diagnostics {
		if d != nil && d.Severity == core.SeverityError {
			return true
		}
	}
	return false
}

func printSummary(w io.Writer, sm statemachine.Statemachine) {
	fmt.Fprintf(w, "State machine: %q\n", sm.Name())
	fmt.Fprintf(w, "Events: %s\n", joinNames(eventNames(sm)))
	fmt.Fprintf(w, "Commands: %s\n", joinNames(commandNames(sm)))
	initRef := sm.Init()
	if initRef == nil {
		fmt.Fprintf(w, "Initial state: <missing>\n")
	} else {
		fmt.Fprintf(w, "Initial state: %q\n", initRef.Text())
	}
	fmt.Fprintf(w, "States:\n")
	for _, st := range sm.States() {
		var actionParts []string
		for _, a := range st.Actions() {
			if a == nil {
				continue
			}
			actionParts = append(actionParts, a.Text())
		}
		actions := "(no actions block)"
		if len(actionParts) > 0 {
			actions = strings.Join(actionParts, ", ")
		}
		fmt.Fprintf(w, "  - %q  actions: %s\n", st.Name(), actions)
		for _, tr := range st.Transitions() {
			ev := ""
			if tr.Event() != nil {
				ev = tr.Event().Text()
			}
			to := ""
			if tr.State() != nil {
				to = tr.State().Text()
			}
			fmt.Fprintf(w, "      %s => %s\n", ev, to)
		}
	}
}

func eventNames(sm statemachine.Statemachine) []string {
	var names []string
	for _, e := range sm.Events() {
		names = append(names, e.Name())
	}
	return names
}

func commandNames(sm statemachine.Statemachine) []string {
	var names []string
	for _, c := range sm.Commands() {
		names = append(names, c.Name())
	}
	return names
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func simulate(w io.Writer, input io.Reader, sm statemachine.Statemachine) error {
	ctx := context.Background()
	init := sm.Init()
	if init == nil {
		return fmt.Errorf("no initialState in document")
	}
	current := init.Ref(ctx)
	if current == nil {
		return fmt.Errorf("initial state reference did not resolve (see linker diagnostics)")
	}
	fmt.Fprintf(w, "Start state: %q\n", current.Name())

	sc := bufio.NewScanner(input)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		next, ok := stepTransition(ctx, current, line)
		if !ok {
			fmt.Fprintf(w, "  no transition on %q from %q\n", line, current.Name())
			continue
		}
		current = next
		fmt.Fprintf(w, "  %q -> %q\n", line, current.Name())
	}
	return sc.Err()
}

func stepTransition(ctx context.Context, st statemachine.State, event string) (statemachine.State, bool) {
	for _, tr := range st.Transitions() {
		ev := tr.Event()
		if ev == nil || ev.Text() != event {
			continue
		}
		target := tr.State()
		if target == nil {
			return nil, false
		}
		next := target.Ref(ctx)
		if next == nil {
			return nil, false
		}
		return next, true
	}
	return nil, false
}
