package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"
)

const versionPageTemplate = "web/version.html.tpl"
const versionPageOutput = "docs/version.html"

// versionPageRow is the presentation-shaped view of a releaseRow for the version page.
type versionPageRow struct {
	Tag             string
	URL             string
	SupportStatus   string
	SupportClass    string
	VersionsDisplay string
	PublishedAt     time.Time
}

// renderVersionPage copies the version page's static assets and renders
// web/version.html.tpl with the given rows into docs/version.html.
func renderVersionPage(rows []releaseRow, generatedAt time.Time) error {
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

	pageRows := make([]versionPageRow, 0, len(rows))
	stableReleasesSeen := 0
	for i, row := range rows {
		versionsDisplay := "No data"
		if len(row.K8sVersions) > 0 {
			versionsDisplay = strings.Join(row.K8sVersions, ", ")
		}

		isRC := strings.Contains(strings.ToLower(row.Tag), "rc")

		supportStatus := "Unsupported"
		supportClass := "diff-del"
		if i > 0 && !isRC {
			stableReleasesSeen++
			switch stableReleasesSeen {
			case 1:
				supportStatus = "Supported"
				supportClass = "diff-add"
			case 2:
				supportStatus = "Best-effort"
				supportClass = "text-muted"
			}
		}
		// master (i == 0), rc releases, and stable releases beyond the latest two fall through to Unsupported.

		pageRows = append(pageRows, versionPageRow{
			Tag:             row.Tag,
			URL:             row.HTMLURL,
			SupportStatus:   supportStatus,
			SupportClass:    supportClass,
			VersionsDisplay: versionsDisplay,
			PublishedAt:     row.PublishedAt,
		})
	}

	tmpl, err := template.ParseFiles(versionPageTemplate)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", versionPageTemplate, err)
	}

	out, err := os.Create(versionPageOutput)
	if err != nil {
		return fmt.Errorf("creating %s: %w", versionPageOutput, err)
	}
	defer closeAndLog(out, versionPageOutput)

	data := struct {
		Rows        []versionPageRow
		GeneratedAt string
	}{
		Rows:        pageRows,
		GeneratedAt: generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
	}

	return tmpl.Execute(out, data)
}

// copyFile copies src to dst, overwriting dst if it already exists.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer closeAndLog(srcFile, src)

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer closeAndLog(dstFile, dst)

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// closeAndLog closes c, logging (rather than returning) any error since callers invoke
// this via defer where an error return can't be propagated.
func closeAndLog(c io.Closer, path string) {
	if err := c.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing %s: %v\n", path, err)
	}
}
