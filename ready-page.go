package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

const readyPageTemplate = "web/ready.html.tpl"
const readyPageOutput = "docs/ready.html"

// readyPageRow is the presentation-shaped view of a readyPRRow for the ready-PRs page.
type readyPageRow struct {
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

// renderReadyPage renders web/ready.html.tpl with the given rows into
// docs/ready.html.
func renderReadyPage(rows []readyPRRow, generatedAt time.Time) error {
	pageRows := make([]readyPageRow, 0, len(rows))
	for _, row := range rows {
		pageRows = append(pageRows, readyPageRow{
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

	tmpl, err := template.ParseFiles(readyPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", readyPageTemplate, err)
	}

	out, err := os.Create(readyPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", readyPageOutput, err)
	}
	defer closeAndLog(out, readyPageOutput)

	data := struct {
		Rows        []readyPageRow
		GeneratedAt string
	}{
		Rows:        pageRows,
		GeneratedAt: generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}
