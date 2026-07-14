package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

const flakyPageTemplate = "web/flaky.html.tpl"
const flakyPageOutput = "docs/flaky.html"

// flakyPageRow is the presentation-shaped view of a flakyTestRow for the flaky-tests page.
type flakyPageRow struct {
	Name                string
	Suite               string
	FlakeRateDisplay    string
	LastFailureRelative string
	LastFailureTitle    string
}

// renderFlakyPage copies the flaky-tests page's static assets and renders
// web/flaky.html.tpl with the given rows into docs/flaky.html.
func renderFlakyPage(rows []flakyTestRow, generatedAt time.Time) error {
	if err := os.MkdirAll("docs/img", 0755); err != nil {
		return fmt.Errorf("creating docs dir: %w", err)
	}
	if err := copyFile("web/dashboard.css", "docs/dashboard.css"); err != nil {
		return fmt.Errorf("copying dashboard.css: %w", err)
	}
	if err := copyFile("web/img/menu.svg", "docs/img/menu.svg"); err != nil {
		return fmt.Errorf("copying menu.svg: %w", err)
	}
	if err := copyFile("web/img/rollouts.png", "docs/img/rollouts.png"); err != nil {
		return fmt.Errorf("copying rollouts.png: %w", err)
	}
	if err := copyFile("web/img/favicon.ico", "docs/img/favicon.ico"); err != nil {
		return fmt.Errorf("copying favicon.ico: %w", err)
	}

	pageRows := make([]flakyPageRow, 0, len(rows))
	for _, row := range rows {
		pageRows = append(pageRows, flakyPageRow{
			Name:                row.Name,
			Suite:               row.Suite,
			FlakeRateDisplay:    fmt.Sprintf("%.0f%%", 100*float64(row.FailCount)/float64(row.SampledRuns)),
			LastFailureRelative: humanize.RelTime(row.LastFailure, generatedAt, "ago", "from now"),
			LastFailureTitle:    row.LastFailure.Format("02 Jan 2006"),
		})
	}

	tmpl, err := template.ParseFiles(flakyPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", flakyPageTemplate, err)
	}

	out, err := os.Create(flakyPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", flakyPageOutput, err)
	}
	defer closeAndLog(out, flakyPageOutput)

	data := struct {
		Rows        []flakyPageRow
		GeneratedAt string
	}{
		Rows:        pageRows,
		GeneratedAt: generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}
