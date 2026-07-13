package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"
)

const indexPageTemplate = "web/index.html.tpl"
const indexPageOutput = "docs/index.html"

// renderIndexPage copies the index page's static assets and renders
// web/index.html.tpl with the given rows into docs/index.html.
func renderIndexPage(rows []releaseRow, generatedAt time.Time) error {
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

	totalReleases := len(rows) - 1

	var lastReleaseTag string
	var releasedOn time.Time
	for _, row := range rows[1:] { // skip master
		if strings.Contains(strings.ToLower(row.Tag), "rc") {
			continue
		}
		lastReleaseTag = row.Tag
		releasedOn = row.PublishedAt
		break
	}

	tmpl, err := template.ParseFiles(indexPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", indexPageTemplate, err)
	}

	out, err := os.Create(indexPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", indexPageOutput, err)
	}
	defer closeAndLog(out, indexPageOutput)

	data := struct {
		TotalReleases int
		LastRelease   string
		ReleasedOn    string
		GeneratedAt   string
	}{
		TotalReleases: totalReleases,
		LastRelease:   lastReleaseTag,
		ReleasedOn:    releasedOn.Format("2 Jan 2006"),
		GeneratedAt:   generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}
