package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Run is the public entry point for the run command.
func Run(args []string) {
	if err := runRun(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	jdPath := fs.String("jd", "", "Job description file path")
	apiKey := fs.String("api-key", "", "Gemini API key")
	model := fs.String("model", "gemini-2.0-flash", "Gemini model")
	contentPath := fs.String("content", "content.json", "Input content JSON")
	templatePath := fs.String("template", "template.tex", "LaTeX template")
	outputTex := fs.String("output-tex", "output.tex", "Intermediate .tex output")
	modifiedPath := fs.String("modified", "modified.json", "Intermediate modified JSON")
	outputDir := fs.String("output-dir", ".", "PDF output directory")
	quiet := fs.Bool("quiet", false, "Suppress pdflatex output")
	fs.Parse(args)

	if strings.TrimSpace(*jdPath) == "" {
		return fmt.Errorf("job description path is required: %w", errors.New("missing --jd"))
	}

	fmt.Fprintln(os.Stdout, "[1/3] Tailoring content with Gemini...")
	tailorArgs := []string{
		"--content", *contentPath,
		"--template", *templatePath,
		"--jd", *jdPath,
		"--output", *modifiedPath,
		"--model", *model,
	}
	if strings.TrimSpace(*apiKey) != "" {
		tailorArgs = append(tailorArgs, "--api-key", *apiKey)
	}
	if err := runTailor(tailorArgs); err != nil {
		return fmt.Errorf("tailor step failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, "[2/3] Applying content to template...")
	applyArgs := []string{
		"--template", *templatePath,
		"--content", *modifiedPath,
		"--output", *outputTex,
	}
	if err := runApply(applyArgs); err != nil {
		return fmt.Errorf("apply step failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, "[3/3] Compiling PDF...")
	compileArgs := []string{
		"--input", *outputTex,
		"--output-dir", *outputDir,
	}
	if *quiet {
		compileArgs = append(compileArgs, "--quiet")
	}
	if err := runCompile(compileArgs); err != nil {
		return fmt.Errorf("compile step failed: %w", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(*outputTex), filepath.Ext(*outputTex))
	outputPDF := filepath.Join(*outputDir, baseName+".pdf")
	if *outputDir == "." {
		outputPDF = "./" + baseName + ".pdf"
	}

	fmt.Fprintf(os.Stdout, "%s Done! Your tailored resume is ready at %s\n", okMark, outputPDF)
	return nil
}
