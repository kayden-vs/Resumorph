# resume-tailor

resume-tailor is a CLI that tailors a LaTeX resume to a job description using the Gemini Flash API, then compiles the result to PDF with pdflatex. It scans %%KEY%% markers, rewrites your content JSON, applies replacements to the template, and produces output.pdf.

## Prerequisites

- Go 1.21+
- pdflatex (TeX Live or MiKTeX)
- Gemini API key

## Build

```bash
go build -o resume-tailor .
```

## Annotate your template.tex

Add %%KEY%% placeholders where content should be rewritten (typically project bullets and optional skills). Header/contact info, Education, Achievements, and project headings/summaries are locked and ignored during tailoring even if you add markers there. If your resume lives in another file, pass `--template` to tailor/run or copy it to template.tex.

Example:

```latex
\section{Summary}
\textbf{%%NAME%%}
\href{mailto:%%EMAIL%%}{%%EMAIL%%}
\begin{itemize}
  \item %%JOB1_BULLET1%%
```

## Usage

One-time setup:

```bash
resume-tailor extract --template template.tex --output content.json
```

Fill in content.json with your current resume text.

For each job application:

```bash
resume-tailor run --jd job_description.txt --api-key YOUR_KEY
```

Or step by step:

```bash
resume-tailor tailor --jd job_description.txt --api-key YOUR_KEY
resume-tailor apply
resume-tailor compile
```

## Tailoring rules

- Header/contact info, Education, Achievements, and project headings/summaries are locked and never sent to the model.
- Technical Skills is add-only: existing text must remain intact and any additions are minimal and strongly implied.
- Project bullets are length-guarded to at most +10% of the original length; if exceeded, the tool trims or falls back to the original.
- Bullet counts are preserved.

## Commands and flags

| Command | Flags | Description |
| --- | --- | --- |
| extract | `--template` (default template.tex)<br>`--output` (default content.json) | Scan template markers and write a skeleton content JSON. |
| tailor | `--content` (default content.json)<br>`--template` (default template.tex)<br>`--jd` (required)<br>`--output` (default modified.json)<br>`--api-key` (default empty; falls back to GEMINI_API_KEY)<br>`--model` (default gemini-2.0-flash) | Call Gemini to tailor JSON values to a job description. |
| apply | `--template` (default template.tex)<br>`--content` (default modified.json)<br>`--output` (default output.tex) | Replace markers in the template with JSON values. |
| compile | `--input` (default output.tex)<br>`--output-dir` (default .)<br>`--runs` (default 2)<br>`--quiet` (default false) | Run pdflatex to produce output.pdf. |
| run | `--jd` (required)<br>`--api-key` (default empty)<br>`--model` (default gemini-2.0-flash)<br>`--content` (default content.json)<br>`--template` (default template.tex)<br>`--output-tex` (default output.tex)<br>`--modified` (default modified.json)<br>`--output-dir` (default .)<br>`--quiet` (default false) | Tailor, apply, and compile in sequence. |

## Tip

Store your API key in your shell profile to avoid typing it each time:

```bash
export GEMINI_API_KEY="your-key"
```
