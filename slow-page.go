package main

import (
	"fmt"
	"html/template"
	"os"
	"time"
)

const slowPageTemplate = "web/slow.html.tpl"
const slowPageOutput = "docs/slow.html"

// slowPageRow is the presentation-shaped view of a slowTestRow for the slow-tests page.
type slowPageRow struct {
	Name            string
	Suite           string
	DurationDisplay string
	FailClass       string
}

// renderSlowPage renders web/slow.html.tpl with the given rows into
// docs/slow.html.
func renderSlowPage(rows []slowTestRow, generatedAt time.Time) error {
	pageRows := make([]slowPageRow, 0, len(rows))
	for _, row := range rows {
		suite := row.Suite
		if suite == "" {
			suite = "-"
		}
		failClass := ""
		if row.Status == "FAIL" {
			failClass = "diff-del"
		}
		pageRows = append(pageRows, slowPageRow{
			Name:            row.Name,
			Suite:           suite,
			DurationDisplay: fmt.Sprintf("%.1fs", row.DurationSeconds),
			FailClass:       failClass,
		})
	}

	tmpl, err := template.ParseFiles(slowPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", slowPageTemplate, err)
	}

	out, err := os.Create(slowPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", slowPageOutput, err)
	}
	defer closeAndLog(out, slowPageOutput)

	data := struct {
		Rows        []slowPageRow
		GeneratedAt string
	}{
		Rows:        pageRows,
		GeneratedAt: generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}
