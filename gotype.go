// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
The gotype command, like the front-end of a Go compiler, parses and
type-checks a single Go package. Errors are reported if the analysis
fails; otherwise gotype is quiet (unless -v is set).

Without a list of paths, gotype reads from standard input, which
must provide a single Go source file defining a complete package.

With a single directory argument, gotype checks the Go files in
that directory, comprising a single package. Use -t to include the
(in-package) _test.go files. Use -x to type check only external
test files.

Otherwise, each path must be the filename of a Go file belonging
to the same package.

Imports are processed by importing directly from the source of
imported packages (default), or by importing from compiled and
installed packages (by setting -c to the respective compiler).

Usage:
	gotype [flags] [path...]

The flags are:
	-t
		include local test files in a directory (ignored if -x is provided)
	-x
		consider only external test files in a directory
	-e
		report all errors (not just the first 10)
	-v
		verbose mode

Flags controlling additional output:
	-ast
		print AST (forces -seq)
	-trace
		print parse trace (forces -seq)
	-comments
		parse comments (ignored unless -ast or -trace is provided)

Examples:

To check the files a.go, b.go, and c.go:

	gotype a.go b.go c.go

To check an entire package including (in-package) tests in the directory dir and print the processed files:

	gotype -t -v dir


To verify the output of a pipe:

	echo "package foo" | gotype

*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	// main operation modes
	testFiles  = flag.Bool("t", false, "include in-package test files in a directory")
	xtestFiles = flag.Bool("x", false, "consider only external test files in a directory")
	allErrors  = flag.Bool("e", false, "report all errors, not just the first 10")
	verbose    = flag.Bool("v", false, "verbose mode")
	compiler   = flag.String("c", "source", "compiler used for installed packages (gc, gccgo, or source)")

	// additional output control
	printAST      = flag.Bool("ast", false, "print AST (forces -seq)")
	printTrace    = flag.Bool("trace", false, "print parse trace (forces -seq)")
	parseComments = flag.Bool("comments", false, "parse comments (ignored unless -ast or -trace is provided)")

	// Support "as you type" typechecking with these flags.
	modifiedFilename = flag.String("modifiedFilename", "", "Path to modified file for as-you-type checking.")
	origFilename     = flag.String("origFilename", "", "Original path to file for as-you-type checking.")
)

var (
	fset       = token.NewFileSet()
	errorCount = 0
	sequential = false
	parserMode parser.Mode
)

func initParserMode() {
	if *allErrors {
		parserMode |= parser.AllErrors
	}
	if *printAST {
		sequential = true
	}
	if *printTrace {
		parserMode |= parser.Trace
		sequential = true
	}
	if *parseComments && (*printAST || *printTrace) {
		parserMode |= parser.ParseComments
	}
}

const usageString = `usage: gotype [flags] [path ...]

The gotype command, like the front-end of a Go compiler, parses and
type-checks a single Go package. Errors are reported if the analysis
fails; otherwise gotype is quiet (unless -v is set).

Without a list of paths, gotype reads from standard input, which
must provide a single Go source file defining a complete package.

With a single directory argument, gotype checks the Go files in
that directory, comprising a single package. Use -t to include the
(in-package) _test.go files. Use -x to type check only external
test files.

Otherwise, each path must be the filename of a Go file belonging
to the same package.

Imports are processed by importing directly from the source of
imported packages.
`

func usage() {
	fmt.Fprintln(os.Stderr, usageString)
	flag.PrintDefaults()
	os.Exit(2)
}

func report(err error) {
	scanner.PrintError(os.Stderr, err)
	if list, ok := err.(scanner.ErrorList); ok {
		errorCount += len(list)
		return
	}
	errorCount++
}

// parse may be called concurrently
func parse(filename string, src interface{}) (*ast.File, error) {
	if *verbose {
		fmt.Println(filename)
	}
	file, err := parser.ParseFile(fset, filename, src, parserMode) // ok to access fset concurrently
	if *printAST {
		ast.Print(fset, file)
	}
	return file, err
}

func parseStdin() (*ast.File, error) {
	src, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return parse("<standard input>", src)
}

func parseFiles(dir string, filenames []string) ([]*ast.File, error) {
	files := make([]*ast.File, len(filenames))
	errors := make([]error, len(filenames))

	var wg sync.WaitGroup
	for i, filename := range filenames {
		wg.Add(1)
		go func(i int, filepath string) {
			defer wg.Done()
			files[i], errors[i] = parse(filepath, nil)
		}(i, filepath.Join(dir, filename))
		if sequential {
			wg.Wait()
		}
	}
	wg.Wait()

	// if there are errors, return the first one for deterministic results
	for _, err := range errors {
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

func parseDir(dir, exclude string, includeTestFiles bool) ([]*ast.File, error) {
	ctxt := build.Default
	pkginfo, err := ctxt.ImportDir(dir, 0)
	if _, nogo := err.(*build.NoGoError); err != nil && !nogo {
		return nil, err
	}

	if *xtestFiles {
		return parseFiles(dir, pkginfo.XTestGoFiles)
	}

	filenames := append(pkginfo.GoFiles, pkginfo.CgoFiles...)
	if includeTestFiles {
		filenames = append(filenames, pkginfo.TestGoFiles...)
	}

	// Remove the excluded file.
	for i, fn := range filenames {
		if fn == exclude {
			filenames = append(filenames[:i], filenames[i+1:]...)
			break
		}
	}
	return parseFiles(dir, filenames)
}

// Return the files to check as a package for "as you type" use cases in an
// editor. This returns all of the files in the same directory as origFilename
// except origFilename plus modifiedFilename.
func getAsYouTypeFiles() ([]*ast.File, error) {
	dir := filepath.Dir(*origFilename)
	base := filepath.Base(*origFilename)
	includeTestFiles := strings.HasSuffix(*origFilename, "_test.go")
	baseFiles, err := parseDir(dir, base, includeTestFiles)
	if err != nil {
		return nil, err
	}
	modifiedFile, err := parse(*modifiedFilename, nil)
	if err != nil {
		return nil, err
	}
	return append(baseFiles, modifiedFile), nil
}

func getPkgFiles(args []string) ([]*ast.File, error) {
	if len(args) == 0 {
		// stdin
		file, err := parseStdin()
		if err != nil {
			return nil, err
		}
		return []*ast.File{file}, nil
	}

	if len(args) == 1 {
		// possibly a directory
		path := args[0]
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return parseDir(path, "", *testFiles)
		}
	}

	// list of files
	return parseFiles("", args)
}

func checkPkgFiles(files []*ast.File) {
	type bailout struct{}

	// if checkPkgFiles is called multiple times, set up conf only once
	conf := types.Config{
		FakeImportC: true,
		Error: func(err error) {
			if !*allErrors && errorCount >= 10 {
				panic(bailout{})
			}
			report(err)
		},

		// Changed because I want to use the srcimporter with go 1.8
		Importer: New(&build.Default, fset, make(map[string]*types.Package)),
		// Changed to work with go 1.8
		Sizes: &types.StdSizes{8, 8},
	}

	defer func() {
		switch p := recover().(type) {
		case nil, bailout:
			// normal return or early exit
		default:
			// re-panic
			panic(p)
		}
	}()

	const path = "pkg" // any non-empty string will do for now
	conf.Check(path, fset, files, nil)
}

func printStats(d time.Duration) {
	fileCount := 0
	lineCount := 0
	fset.Iterate(func(f *token.File) bool {
		fileCount++
		lineCount += f.LineCount()
		return true
	})

	fmt.Printf(
		"%s (%d files, %d lines, %d lines/s)\n",
		d, fileCount, lineCount, int64(float64(lineCount)/d.Seconds()),
	)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	initParserMode()

	start := time.Now()

	var files []*ast.File
	var err error

	if *modifiedFilename != "" || *origFilename != "" {
		if *modifiedFilename == "" || *origFilename == "" {
			report(errors.New("Both or neither modifiedFilename and origFilename must be specified."))
			os.Exit(2)
		}

		files, err = getAsYouTypeFiles()
	} else {
		files, err = getPkgFiles(flag.Args())
	}
	if err != nil {
		report(err)
		os.Exit(2)
	}

	checkPkgFiles(files)
	if errorCount > 0 {
		os.Exit(2)
	}

	if *verbose {
		printStats(time.Since(start))
	}
}
