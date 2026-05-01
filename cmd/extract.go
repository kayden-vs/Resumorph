package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

// Extract is the public entry point for the extract command.
func Extract(args []string) {
	if err := runExtract(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	templatePath := fs.String("template", "template.tex", "Path to the annotated LaTeX template")
	outputPath := fs.String("output", "content.json", "Path to write the skeleton JSON")
	fs.Parse(args)

	content, err := os.ReadFile(*templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template file not found: %w", errors.New(*templatePath))
		}
		return fmt.Errorf("read template file: %w", err)
	}

	matches := markerRe.FindAllStringSubmatch(string(content), -1)
	if len(matches) == 0 {
		baseErr := errors.New("no markers")
		return fmt.Errorf("no %%KEY%% markers found in %s \u2014 have you annotated your template?: %w", *templatePath, baseErr)
	}

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		seen[match[1]] = true
	}

	if len(seen) == 0 {
		baseErr := errors.New("no markers")
		return fmt.Errorf("no %%KEY%% markers found in %s \u2014 have you annotated your template?: %w", *templatePath, baseErr)
	}

	data := make(map[string]string, len(seen))
	for key := range seen {
		data[key] = ""
	}

	if _, err := os.Stat(*outputPath); err == nil {
		fmt.Fprintln(os.Stdout, "Warning: output file already exists and will be overwritten. Press Enter to continue or Ctrl+C to cancel.")
		if _, scanErr := fmt.Scanln(); scanErr != nil {
			return fmt.Errorf("read confirmation: %w", scanErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check output file: %w", err)
	}

	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skeleton JSON: %w", err)
	}

	if err := os.WriteFile(*outputPath, payload, 0644); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s Found %d unique markers in %s\n", okMark, len(seen), *templatePath)
	fmt.Fprintf(os.Stdout, "%s Written skeleton to %s\n\n", okMark, *outputPath)
	fmt.Fprintf(os.Stdout, "Next step: open %s and fill in your current resume values for each key.\n", *outputPath)
	fmt.Fprintln(os.Stdout, "Then run: resume-tailor tailor --jd <job_description.txt>")

	return nil
}
