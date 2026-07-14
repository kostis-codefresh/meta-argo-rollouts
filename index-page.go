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

// renderIndexPage renders web/index.html.tpl with the given rows into
// docs/index.html.
func renderIndexPage(rows []releaseRow, generatedAt time.Time, criticalCount, readyCount, docCount int) error {
	totalReleases := len(rows) - 1

	lastReleaseTag, releasedOn := latestStableRelease(rows)

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
		TotalReleases   int
		LastRelease     string
		ReleasedOn      string
		GeneratedAt     string
		CriticalPRCount int
		ReadyPRCount    int
		DocPRCount      int
	}{
		TotalReleases:   totalReleases,
		LastRelease:     lastReleaseTag,
		ReleasedOn:      releasedOn.Format("2 Jan 2006"),
		GeneratedAt:     generatedAt.UTC().Format("02 Jan 2006 15:04 MST"),
		CriticalPRCount: criticalCount,
		ReadyPRCount:    readyCount,
		DocPRCount:      docCount,
	}

	return tmpl.Execute(out, data)
}

// latestStableRelease returns the tag and publish date of the first
// non-release-candidate entry after rows[0] (master), or ("", zero time) if
// every remaining row is an RC.
func latestStableRelease(rows []releaseRow) (tag string, publishedAt time.Time) {
	for _, row := range rows[1:] { // skip master
		if strings.Contains(strings.ToLower(row.Tag), "rc") {
			continue
		}
		return row.Tag, row.PublishedAt
	}
	return "", time.Time{}
}
