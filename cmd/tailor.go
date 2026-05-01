package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type generationConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// Tailor is the public entry point for the tailor command.
func Tailor(args []string) {
	if err := runTailor(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTailor(args []string) error {
	fs := flag.NewFlagSet("tailor", flag.ExitOnError)
	contentPath := fs.String("content", "content.json", "Path to filled content JSON")
	templatePath := fs.String("template", "template.tex", "Path to annotated LaTeX template")
	jdPath := fs.String("jd", "", "Path to plain-text job description file")
	outputPath := fs.String("output", "modified.json", "Path to write the LLM-modified JSON")
	apiKey := fs.String("api-key", "", "Gemini API key. If empty, read from env var GEMINI_API_KEY")
	model := fs.String("model", "gemini-2.0-flash", "Gemini model name")
	fs.Parse(args)

	if strings.TrimSpace(*jdPath) == "" {
		return fmt.Errorf("job description path is required: %w", errors.New("missing --jd"))
	}

	resolvedKey := strings.TrimSpace(*apiKey)
	if resolvedKey == "" {
		resolvedKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	}
	if resolvedKey == "" {
		baseErr := errors.New("missing Gemini API key")
		return fmt.Errorf("Gemini API key not provided. Use --api-key or set GEMINI_API_KEY env var: %w", baseErr)
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

	markerInfo, err := parseTemplateMarkers(*templatePath)
	if err != nil {
		return fmt.Errorf("parse template markers: %w", err)
	}

	rules := buildTailorRules(content, markerInfo)
	if len(rules.Tailorable) == 0 {
		return fmt.Errorf("no tailorable fields found in template: %w", errors.New("no tailorable keys"))
	}

	jdBytes, err := os.ReadFile(*jdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("job description file not found: %w", errors.New(*jdPath))
		}
		return fmt.Errorf("read job description file: %w", err)
	}

	prettyContent, err := json.MarshalIndent(rules.Tailorable, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal content JSON for prompt: %w", err)
	}

	prompt := fmt.Sprintf(strings.ReplaceAll(promptTemplate, "{{EMDASH}}", "\u2014"), string(prettyContent), string(jdBytes))

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
		GenerationConfig: generationConfig{
			Temperature:      0.3,
			ResponseMimeType: "application/json",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal Gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", *model, resolvedKey)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create Gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send Gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read Gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr geminiResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error != nil {
			return fmt.Errorf("Gemini API error (HTTP %d): %w", resp.StatusCode, errors.New(apiErr.Error.Message))
		}
		return fmt.Errorf("Gemini API error (HTTP %d): %w", resp.StatusCode, errors.New(string(respBody)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("parse Gemini response: %w", err)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		baseErr := errors.New("empty candidates")
		return fmt.Errorf("Gemini returned empty response. Try again: %w", baseErr)
	}

	text := parsed.Candidates[0].Content.Parts[0].Text
	cleaned := stripCodeFences(text)

	var modified map[string]string
	if err := json.Unmarshal([]byte(cleaned), &modified); err != nil {
		return fmt.Errorf("parse tailored JSON: %w", err)
	}

	for key := range rules.Tailorable {
		if _, ok := modified[key]; !ok {
			baseErr := errors.New("missing key")
			return fmt.Errorf("LLM response is missing key %q \u2014 the model did not follow instructions. Try running tailor again: %w", key, baseErr)
		}
	}

	for key := range modified {
		if _, ok := rules.Tailorable[key]; !ok {
			fmt.Fprintf(os.Stdout, "Warning: LLM added unexpected key %q \u2014 it will be ignored.\n", key)
			delete(modified, key)
		}
	}

	enforceSkillsAddOnly(modified, content, rules.SkillKeys)
	enforceProjectBulletLength(modified, content, rules.ProjectBulletKeys, 1.10)

	for key, value := range rules.Locked {
		modified[key] = value
	}

	outputBytes, err := json.MarshalIndent(modified, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal modified JSON: %w", err)
	}

	if err := os.WriteFile(*outputPath, outputBytes, 0644); err != nil {
		return fmt.Errorf("write modified JSON: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s Gemini tailored your resume content successfully\n", okMark)
	fmt.Fprintf(os.Stdout, "%s Written to %s\n\n", okMark, *outputPath)
	fmt.Fprintln(os.Stdout, "Next step: resume-tailor apply")

	return nil
}

func stripCodeFences(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) <= 2 {
		return trimmed
	}

	// Drop the opening fence line and an optional closing fence line.
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type tailorRules struct {
	Tailorable        map[string]string
	Locked            map[string]string
	SkillKeys         map[string]bool
	ProjectBulletKeys map[string]bool
}

func buildTailorRules(content map[string]string, markerInfo map[string]markerInfo) tailorRules {
	rules := tailorRules{
		Tailorable:        make(map[string]string),
		Locked:            make(map[string]string),
		SkillKeys:         make(map[string]bool),
		ProjectBulletKeys: make(map[string]bool),
	}

	for key, value := range content {
		info, ok := markerInfo[key]
		section := ""
		if ok {
			section = info.Section
		}

		if isHeaderSection(section) || isEducationSection(section) || isAchievementsSection(section) {
			rules.Locked[key] = value
			continue
		}

		if isProjectsSection(section) {
			if info.InResumeItem {
				rules.ProjectBulletKeys[key] = true
				rules.Tailorable[key] = value
			} else {
				rules.Locked[key] = value
			}
			continue
		}

		if isSkillsSection(section) {
			rules.SkillKeys[key] = true
		}

		rules.Tailorable[key] = value
	}

	return rules
}

func enforceSkillsAddOnly(modified, original map[string]string, skillKeys map[string]bool) {
	for key := range skillKeys {
		orig, ok := original[key]
		if !ok {
			continue
		}
		mod, ok := modified[key]
		if !ok {
			continue
		}

		if strings.TrimSpace(mod) == strings.TrimSpace(orig) {
			continue
		}

		if !strings.HasPrefix(normalizeSkillValue(mod), normalizeSkillValue(orig)) {
			fmt.Fprintf(os.Stdout, "Warning: skills key %q was modified beyond allowed additions; keeping original value.\n", key)
			modified[key] = orig
		}
	}
}

func normalizeSkillValue(value string) string {
	token := "__LINEBREAK__"
	value = strings.ReplaceAll(value, "\\\\", token)
	parts := strings.Fields(value)
	normalized := strings.Join(parts, " ")
	return strings.ReplaceAll(normalized, token, "\\\\")
}

func enforceProjectBulletLength(modified, original map[string]string, bulletKeys map[string]bool, growthRatio float64) {
	for key := range bulletKeys {
		orig, ok := original[key]
		if !ok {
			continue
		}
		mod, ok := modified[key]
		if !ok {
			continue
		}

		maxLen := maxContentLength(orig, growthRatio)
		if runeCount(mod) <= maxLen {
			continue
		}

		trimmed := truncateToWordBoundary(mod, maxLen)
		if trimmed == "" || !bracesBalanced(trimmed) {
			fmt.Fprintf(os.Stdout, "Warning: project bullet %q exceeded length limit; keeping original value.\n", key)
			modified[key] = orig
			continue
		}

		fmt.Fprintf(os.Stdout, "Warning: project bullet %q exceeded length limit; truncated to preserve formatting.\n", key)
		modified[key] = trimmed
	}
}

func maxContentLength(original string, growthRatio float64) int {
	base := runeCount(original)
	if base == 0 {
		return 0
	}
	return int(math.Ceil(float64(base) * growthRatio))
}

func truncateToWordBoundary(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}

	cut := maxLen
	for i := maxLen; i > 0; i-- {
		if unicode.IsSpace(runes[i-1]) {
			cut = i - 1
			break
		}
	}
	if cut <= 0 {
		cut = maxLen
	}

	return strings.TrimSpace(string(runes[:cut]))
}

func bracesBalanced(text string) bool {
	depth := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' {
			if i+1 < len(text) && (text[i+1] == '{' || text[i+1] == '}') {
				i++
				continue
			}
		}

		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func runeCount(value string) int {
	return utf8.RuneCountInString(value)
}

const promptTemplate = `You are an expert resume writer and career coach. Your task is to tailor a resume's content to match a specific job description, making it more likely to pass ATS screening and impress human reviewers.

You will receive:
1. A JSON object representing the current resume content, where each key is a section/field identifier and each value is the current text for that field.
2. A job description to tailor the resume toward.

Your job is to rewrite the VALUES in the JSON to better match the job description. Follow these rules strictly:

RULES:
- Return ONLY a valid JSON object. No explanation, no markdown, no code fences, no preamble.
- Keep EVERY key from the input JSON. Do not add new keys. Do not remove keys.
- Preserve all LaTeX formatting commands in the values (e.g., \textbf{}, \href{}, \textit{}, bullet content, etc.). Only change the human-readable text portions.
- The input JSON already EXCLUDES header/contact info, Education, Achievements, and project headings/summaries. Do not add those fields back or invent new keys.
- Do not change values for fields that are factual and non-tailorable (e.g., your name, email, phone, address, LinkedIn URL, GitHub URL, GPA, degree name, university name, graduation date). If any appear, keep them unchanged.
- For project bullet points, rewrite them to emphasize skills, tools, and outcomes that are most relevant to the job description. Use strong action verbs.
- Project bullet points must stay within 10% of their original length. If you need to add details, replace wording rather than lengthening.
- Keep bullet point counts the same {{EMDASH}} do not add or remove bullets.
- For the Technical Skills section, ONLY append minimal additions strongly implied by current skills/projects. Do not remove or reorder existing text. Do not add complex or very difficult skills.
- For the summary or objective (if present), rewrite it to directly speak to the role.
- Values must remain reasonable in length. Do not expand a short bullet into a paragraph.

CURRENT RESUME CONTENT JSON:
%s

JOB DESCRIPTION:
%s

Return the tailored JSON now:
`
