package main

import (
	"fmt"
	"regexp"
	"strings"
)

var markdownWhitespace = regexp.MustCompile(`\s+`)

type Report struct {
	File        FileInfo      `json:"file"`
	Summary     Summary       `json:"summary"`
	Checks      []Check       `json:"checks"`
	Unsupported []Unsupported `json:"unsupported"`
}

type FileInfo struct {
	Path     string `json:"path"`
	RealPath string `json:"realPath,omitempty"`
	Size     int64  `json:"size"`
}

type Summary struct {
	DatasetCount int `json:"datasetCount"`
	ObjectCount  int `json:"objectCount"`
	Pass         int `json:"pass"`
	Warn         int `json:"warn"`
	Fail         int `json:"fail"`
}

type Check struct {
	ID       string         `json:"id"`
	Level    string         `json:"level"`
	Status   string         `json:"status"`
	Message  string         `json:"message"`
	Dataset  string         `json:"dataset,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type Unsupported struct {
	Dataset string `json:"dataset"`
	Kind    string `json:"kind,omitempty"`
	Reason  string `json:"reason"`
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# UDBX Validation Report\n\n")
	fmt.Fprintf(&b, "- File: `%s`\n", report.File.Path)
	fmt.Fprintf(&b, "- Datasets: `%d`\n", report.Summary.DatasetCount)
	fmt.Fprintf(&b, "- Objects: `%d`\n", report.Summary.ObjectCount)
	fmt.Fprintf(&b, "- Pass/Warn/Fail: `%d` / `%d` / `%d`\n\n", report.Summary.Pass, report.Summary.Warn, report.Summary.Fail)
	fmt.Fprintf(&b, "| ID | Status | Level | Dataset | Message |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|\n")
	for _, check := range report.Checks {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %s |\n", escapeMarkdownCell(check.ID), escapeMarkdownCell(check.Status), escapeMarkdownCell(check.Level), escapeMarkdownCell(check.Dataset), escapeMarkdownCell(check.Message))
	}
	return b.String()
}

func escapeMarkdownCell(value string) string {
	value = markdownWhitespace.ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "`", "'")
	return strings.TrimSpace(value)
}
