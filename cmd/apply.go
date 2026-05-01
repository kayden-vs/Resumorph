package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Apply is the public entry point for the apply command.
func Apply(args []string) {
	if err := runApply(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	templatePath := fs.String("template", "template.tex", "Path to annotated template")
	contentPath := fs.String("content", "modified.json", "Path to JSON with replacement values")
	outputPath := fs.String("output", "output.tex", "Path to write the filled-in LaTeX file")
	fs.Parse(args)

	templateBytes, err := os.ReadFile(*templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template file not found: %w", errors.New(*templatePath))
		}
		return fmt.Errorf("read template file: %w", err)
	}

	contentBytes, err := os.ReadFile(*contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("content file not found: %w", errors.New(*contentPath))
		}
		return fmt.Errorf("read content file: %w", err)
	}

	var content map[string]string
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("parse content JSON: %w", err)
	}

	templateContent := string(templateBytes)
	result := templateContent
	replacedCount := 0

	for key, value := range content {
		marker := "%%" + key + "%%"
		if strings.Contains(result, marker) {
			replacedCount++
		} else {
			debugf("JSON key %q not found in template.", key)
		}
		result = strings.ReplaceAll(result, marker, value)
	}

	unresolved := markerRe.FindAllStringSubmatch(result, -1)
	for _, match := range unresolved {
		if len(match) < 2 {
			continue
		}
		fmt.Fprintf(os.Stdout, "Warning: marker %%%%"+match[1]+"%%%% was not replaced \u2014 no value found in JSON.\n")
	}

	if err := os.WriteFile(*outputPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s Applied %d replacements to %s\n", okMark, replacedCount, *templatePath)
	fmt.Fprintf(os.Stdout, "%s Written to %s\n\n", okMark, *outputPath)
	fmt.Fprintln(os.Stdout, "Next step: resume-tailor compile")

	return nil
}

func debugf(format string, args ...any) {
	if strings.TrimSpace(os.Getenv("RESUME_TAILOR_DEBUG")) == "" {
		return
	}
	fmt.Fprintf(os.Stdout, "Debug: "+format+"\n", args...)
}
