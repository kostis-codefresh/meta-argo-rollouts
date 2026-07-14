package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

const docPageTemplate = "web/nocodeprs.html.tpl"
const docPageOutput = "docs/nocodeprs.html"

// docPageRow is the presentation-shaped view of a docPRRow for the Doc PRs page.
type docPageRow struct {
	Number         int
	URL            string
	Author         string
	AuthorURL      string
	Title          string
	OpenedRelative string
	OpenedTitle    string
	Additions      int
	Deletions      int
}

// renderDocPage renders web/nocodeprs.html.tpl with the given rows into
// docs/nocodeprs.html.
func renderDocPage(rows []docPRRow, generatedAt time.Time) error {
	pageRows := make([]docPageRow, 0, len(rows))
	for _, row := range rows {
		pageRows = append(pageRows, docPageRow{
			Number:         row.Number,
			URL:            row.HTMLURL,
			Author:         row.Author,
			AuthorURL:      "https://github.com/" + row.Author,
			Title:          row.Title,
			OpenedRelative: humanize.RelTime(row.CreatedAt, generatedAt, "ago", "from now"),
			OpenedTitle:    row.CreatedAt.Format("02 Jan 2006"),
			Additions:      row.Additions,
			Deletions:      row.Deletions,
		})
	}

	tmpl, err := template.ParseFiles(docPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", docPageTemplate, err)
	}

	out, err := os.Create(docPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", docPageOutput, err)
	}
	defer closeAndLog(out, docPageOutput)

	data := struct {
		Rows        []docPageRow
		GeneratedAt string
	}{
		Rows:        pageRows,
		GeneratedAt: generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}
