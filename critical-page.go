package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

const criticalPageTemplate = "web/critical.html.tpl"
const criticalPageOutput = "docs/critical.html"

// criticalPageRow is the presentation-shaped view of a criticalPRRow for the critical-PRs page.
type criticalPageRow struct {
	Number         int
	URL            string
	Author         string
	Title          string
	OpenedRelative string
	OpenedTitle    string
	Additions      int
	Deletions      int
}

// renderCriticalPage copies the critical page's static assets and renders
// web/critical.html.tpl with the given rows into docs/critical.html.
func renderCriticalPage(rows []criticalPRRow, generatedAt time.Time) error {
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

	pageRows := make([]criticalPageRow, 0, len(rows))
	for _, row := range rows {
		pageRows = append(pageRows, criticalPageRow{
			Number:         row.Number,
			URL:            row.HTMLURL,
			Author:         row.Author,
			Title:          row.Title,
			OpenedRelative: humanize.RelTime(row.CreatedAt, generatedAt, "ago", "from now"),
			OpenedTitle:    row.CreatedAt.Format("02 Jan 2006"),
			Additions:      row.Additions,
			Deletions:      row.Deletions,
		})
	}

	tmpl, err := template.ParseFiles(criticalPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", criticalPageTemplate, err)
	}

	out, err := os.Create(criticalPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", criticalPageOutput, err)
	}
	defer closeAndLog(out, criticalPageOutput)

	data := struct {
		Rows        []criticalPageRow
		GeneratedAt string
	}{
		Rows:        pageRows,
		GeneratedAt: generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}
