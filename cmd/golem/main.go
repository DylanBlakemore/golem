package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dylanblakemore/golem/internal/ast"
	"github.com/dylanblakemore/golem/internal/checker"
	"github.com/dylanblakemore/golem/internal/codegen"
	"github.com/dylanblakemore/golem/internal/desugar"
	"github.com/dylanblakemore/golem/internal/diagnostic"
	"github.com/dylanblakemore/golem/internal/goloader"
	"github.com/dylanblakemore/golem/internal/lexer"
	"github.com/dylanblakemore/golem/internal/parser"
	"github.com/dylanblakemore/golem/internal/resolver"
	"github.com/dylanblakemore/golem/internal/span"
)

const (
	minArgs  = 2
	dirPerm  = 0o750
	filePerm = 0o600
)

func main() {
	if len(os.Args) < minArgs {
		fmt.Fprintln(os.Stderr, "Usage: golem <command> [arguments]")
		fmt.Fprintln(os.Stderr, "Commands: build, run")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		os.Exit(buildCmd(os.Args[2:]))
	case "run":
		os.Exit(runCmd(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func buildCmd(args []string) int {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "print pipeline timing info")
	_ = fs.Parse(args)

	files, err := discoverGolemFiles(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error discovering files: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no .golem files found")
		return 1
	}

	if err := os.MkdirAll("build", dirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "error creating build directory: %v\n", err)
		return 1
	}

	loader := goloader.New()

	hadErrors := false
	for _, file := range files {
		if !compileFile(file, *verbose, loader) {
			hadErrors = true
		}
	}
	if hadErrors {
		return 1
	}

	// Invoke go build on the generated files.
	start := time.Now()
	cmd := exec.Command("go", "build", "-o", "build/golem-out", "./build/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "go build failed: %v\n", err)
		return 1
	}
	if *verbose {
		fmt.Fprintf(os.Stderr, "[go build] %s\n", time.Since(start))
	}

	return 0
}

func runCmd(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "print pipeline timing info")
	_ = fs.Parse(args)

	buildArgs := []string{}
	if *verbose {
		buildArgs = append(buildArgs, "--verbose")
	}
	if code := buildCmd(buildArgs); code != 0 {
		return code
	}

	binary := filepath.Join("build", "golem-out")
	cmd := exec.Command(binary, fs.Args()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error running binary: %v\n", err)
		return 1
	}
	return 0
}

// discoverGolemFiles finds all .golem files in the given directory.
func discoverGolemFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != root && (info.Name() == "build" || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(path, ".golem") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// compileFile runs the full compilation pipeline on a single .golem file.
//
//nolint:cyclop // pipeline orchestration is naturally sequential
func compileFile(file string, verbose bool, loader *goloader.Loader) bool {
	source, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", file, err)
		return false
	}
	src := string(source)

	mod, ok := frontendPipeline(file, src, verbose, loader)
	if !ok {
		return false
	}

	result := timedDesugar(file, mod, verbose)

	out, err := timedCodegen(file, result.Module, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error for %s: %v\n", file, err)
		return false
	}

	return writeOutput(file, out)
}

// frontendPipeline runs lex, parse, resolve, and type check.
func frontendPipeline(file, src string, verbose bool, loader *goloader.Loader) (*ast.Module, bool) {
	start := time.Now()
	l := lexer.New(src, file)
	tokens := l.Tokenize()
	if verbose {
		fmt.Fprintf(os.Stderr, "[lex] %s: %s\n", file, time.Since(start))
	}

	start = time.Now()
	p := parser.New(tokens, file)
	mod, perrs := p.Parse()
	if verbose {
		fmt.Fprintf(os.Stderr, "[parse] %s: %s\n", file, time.Since(start))
	}
	if len(perrs) > 0 {
		printParseErrors(perrs, "parse", src)
		return nil, false
	}

	start = time.Now()
	res, rerrs := resolver.Resolve(mod)
	if verbose {
		fmt.Fprintf(os.Stderr, "[resolve] %s: %s\n", file, time.Since(start))
	}
	if len(rerrs) > 0 {
		printResolveErrors(rerrs, "resolve", src)
		return nil, false
	}

	start = time.Now()
	info, cerrs := checker.CheckWithLoader(mod, res, loader)
	if verbose {
		fmt.Fprintf(os.Stderr, "[check] %s: %s\n", file, time.Since(start))
	}
	if len(info.Warnings) > 0 {
		printCheckWarnings(info.Warnings, src)
	}
	if len(cerrs) > 0 {
		printCheckErrors(cerrs, "type", src)
		return nil, false
	}

	return mod, true
}

func timedDesugar(file string, mod *ast.Module, verbose bool) *desugar.Result {
	start := time.Now()
	result := desugar.Desugar(mod)
	if verbose {
		fmt.Fprintf(os.Stderr, "[desugar] %s: %s\n", file, time.Since(start))
	}
	return result
}

func timedCodegen(file string, mod *ast.Module, verbose bool) ([]byte, error) {
	start := time.Now()
	out, err := codegen.Generate(mod, file)
	if verbose {
		fmt.Fprintf(os.Stderr, "[codegen] %s: %s\n", file, time.Since(start))
	}
	return out, err
}

func writeOutput(file string, out []byte) bool {
	outFile := golemOutputPath(file)
	outDir := filepath.Dir(outFile)
	if err := os.MkdirAll(outDir, dirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", outDir, err)
		return false
	}
	if err := os.WriteFile(outFile, out, filePerm); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outFile, err)
		return false
	}
	return true
}

// golemOutputPath converts a .golem source path to a build/.golem.go output path.
func golemOutputPath(sourcePath string) string {
	base := strings.TrimSuffix(sourcePath, ".golem")
	return filepath.Join("build", base+".golem.go")
}

func toDiagnostics(spans []span.Span, messages []string, phase string) []diagnostic.Diagnostic {
	diags := make([]diagnostic.Diagnostic, len(spans))
	for i := range spans {
		diags[i] = diagnostic.Diagnostic{
			Span:    spans[i],
			Message: messages[i],
			Phase:   phase,
		}
	}
	return diags
}

func printParseErrors(errs []parser.Error, phase, source string) {
	spans := make([]span.Span, len(errs))
	msgs := make([]string, len(errs))
	for i, e := range errs {
		spans[i] = e.Span
		msgs[i] = e.Message
	}
	fmt.Fprint(os.Stderr, diagnostic.FormatDiagnostics(toDiagnostics(spans, msgs, phase), source))
}

func printResolveErrors(errs []resolver.Error, phase, source string) {
	spans := make([]span.Span, len(errs))
	msgs := make([]string, len(errs))
	for i, e := range errs {
		spans[i] = e.Span
		msgs[i] = e.Message
	}
	fmt.Fprint(os.Stderr, diagnostic.FormatDiagnostics(toDiagnostics(spans, msgs, phase), source))
}

func printCheckWarnings(warnings []checker.Warning, source string) {
	spans := make([]span.Span, len(warnings))
	msgs := make([]string, len(warnings))
	for i, w := range warnings {
		spans[i] = w.Span
		msgs[i] = w.Message
	}
	diags := toDiagnostics(spans, msgs, "type")
	for i := range diags {
		diags[i].Severity = "warning"
	}
	fmt.Fprint(os.Stderr, diagnostic.FormatDiagnostics(diags, source))
}

func printCheckErrors(errs []checker.Error, phase, source string) {
	spans := make([]span.Span, len(errs))
	msgs := make([]string, len(errs))
	for i, e := range errs {
		spans[i] = e.Span
		msgs[i] = e.Message
	}
	fmt.Fprint(os.Stderr, diagnostic.FormatDiagnostics(toDiagnostics(spans, msgs, phase), source))
}
