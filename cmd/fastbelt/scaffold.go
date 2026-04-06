// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"typefox.dev/fastbelt/internal/scaffold"
)

func runScaffoldCLI(args []string) error {
	fs := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	modulePath := fs.String("module", "", "module path for go mod init (use this or -package, not both)")
	packagePath := fs.String("package", "", "import path for a new package inside an existing module (requires go.mod; use this or -module, not both)")
	language := fs.String("language", "", "human-readable language name (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s scaffold -module <path> -language <name>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s scaffold -package <import-path> -language <name>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Module mode (-module): creates a new Go module under a directory named after the final\n"+
			"segment of -module (for example, -module=example.com/acme/foo creates ./foo/).\n\n")
		fmt.Fprintf(os.Stderr, "Package mode (-package): requires go.mod in the current directory or a parent. Writes\n"+
			"the scaffold under the directory that matches the import path relative to the module path\n"+
			"(for example, module example.com/proj and -package=example.com/proj/pkg/mylang creates ./pkg/mylang/).\n"+
			"Does not run go mod init.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSee also: %s help\n", os.Args[0])
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *language == "" {
		fs.Usage()
		return fmt.Errorf("-language is required")
	}
	if *modulePath != "" && *packagePath != "" {
		fs.Usage()
		return fmt.Errorf("use either -module or -package, not both")
	}
	if *modulePath == "" && *packagePath == "" {
		fs.Usage()
		return fmt.Errorf("one of -module or -package is required")
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if *packagePath != "" {
		modRoot, pkgDir, resolveErr := scaffold.ResolvePackageScaffoldDir(wd, *packagePath)
		if resolveErr != nil {
			return resolveErr
		}
		if err := scaffold.RunPackage(modRoot, pkgDir, *packagePath, *language); err != nil {
			return err
		}
		fmt.Printf("Scaffolded package at %s\n", pkgDir)
		return nil
	}

	outDir := filepath.Join(wd, filepath.Base(*modulePath))
	if err := scaffold.RunModule(outDir, *modulePath, *language); err != nil {
		return err
	}
	fmt.Printf("Scaffolded module at %s\n", outDir)
	return nil
}
