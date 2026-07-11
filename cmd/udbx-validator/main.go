package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

var stdout io.Writer = os.Stdout
var stderr io.Writer = os.Stderr

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("udbx-validator", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "markdown", "output format: markdown or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: udbx-validator [--format markdown|json] <file.udbx>")
		return 2
	}
	if *format != "json" && *format != "markdown" {
		fmt.Fprintf(stderr, "unsupported format: %s\n", *format)
		return 2
	}

	report, code := ValidateFile(fs.Arg(0))
	switch *format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "failed to write json report: %v\n", err)
			return 2
		}
	case "markdown":
		if _, err := fmt.Fprint(stdout, RenderMarkdown(report)); err != nil {
			fmt.Fprintf(stderr, "failed to write markdown report: %v\n", err)
			return 2
		}
	}
	return code
}
