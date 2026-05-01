package main

import (
	"errors"
	"fmt"
	"os"

	"resume-tailor/cmd"
)

func main() {
	if len(os.Args) < 2 {
		err := fmt.Errorf("missing command: %w", errors.New("no subcommand provided"))
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "extract":
		cmd.Extract(os.Args[2:])
	case "tailor":
		cmd.Tailor(os.Args[2:])
	case "apply":
		cmd.Apply(os.Args[2:])
	case "compile":
		cmd.Compile(os.Args[2:])
	case "run":
		cmd.Run(os.Args[2:])
	case "--help", "-h", "help":
		printUsage()
	default:
		err := fmt.Errorf("unknown command %q: %w", os.Args[1], errors.New("unrecognized command"))
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Usage: resume-tailor <command> [flags]

Commands:
  extract   Scan template.tex for %%KEY%% markers, write skeleton content.json
  tailor    Call Gemini API to rewrite content.json for a job description
  apply     Inject modified JSON values into template.tex, write output.tex
  compile   Run pdflatex on output.tex to produce output.pdf
	run       Run tailor -> apply -> compile in sequence (full pipeline)

Run resume-tailor <command> --help for flags of each command.
`)
}
