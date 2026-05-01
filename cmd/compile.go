package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Compile is the public entry point for the compile command.
func Compile(args []string) {
	if err := runCompile(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCompile(args []string) error {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	inputPath := fs.String("input", "output.tex", "Path to the .tex file to compile")
	outputDir := fs.String("output-dir", ".", "Directory where PDF and aux files will be written")
	runs := fs.Int("runs", 2, "Number of pdflatex passes")
	quiet := fs.Bool("quiet", false, "Suppress pdflatex stdout (errors still shown)")
	fs.Parse(args)

	if _, err := exec.LookPath("pdflatex"); err != nil {
		baseErr := errors.New("pdflatex not found")
		return fmt.Errorf("pdflatex not found in PATH.\nInstall a LaTeX distribution:\n  macOS:   brew install --cask mactex  (or basictex)\n  Ubuntu:  sudo apt-get install texlive-latex-base\n  Windows: https://miktex.org/download: %w", baseErr)
	}

	if _, err := os.Stat(*inputPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input file not found: %w", errors.New(*inputPath))
		}
		return fmt.Errorf("check input file: %w", err)
	}

	if *outputDir != "." {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	if *runs <= 0 {
		return fmt.Errorf("runs must be positive: %w", errors.New("invalid runs"))
	}

	for i := 1; i <= *runs; i++ {
		fmt.Fprintf(os.Stdout, "Running pdflatex (pass %d/%d)...\n", i, *runs)
		cmdArgs := []string{"-interaction=nonstopmode", "-output-directory", *outputDir, *inputPath}
		cmd := exec.Command("pdflatex", cmdArgs...)
		if !*quiet {
			cmd.Stdout = os.Stdout
		}
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pdflatex failed on pass %d. Check the output above for LaTeX errors.\nTip: Look for lines starting with \"!\" in the pdflatex output: %w", i, err)
		}
	}

	baseName := strings.TrimSuffix(filepath.Base(*inputPath), filepath.Ext(*inputPath))
	pdfPath := filepath.Join(*outputDir, baseName+".pdf")
	displayPath := pdfPath
	if *outputDir == "." {
		displayPath = "./" + baseName + ".pdf"
	}
	if _, err := os.Stat(pdfPath); err != nil {
		fmt.Fprintf(os.Stdout, "Warning: expected PDF %s was not found after compilation.\n", displayPath)
	}

	fmt.Fprintf(os.Stdout, "%s Compilation successful (%d passes)\n", okMark, *runs)
	fmt.Fprintf(os.Stdout, "%s PDF written to %s\n", okMark, displayPath)

	return nil
}
