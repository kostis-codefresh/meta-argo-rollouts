package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

const versionPageTemplate = "web/version.html.tpl"
const versionPageOutput = "docs/version.html"

// versionPageRow is the presentation-shaped view of a releaseRow for the version page.
type versionPageRow struct {
	Tag               string
	URL               string
	SupportStatus     string
	SupportClass      string
	VersionsDisplay   string
	VersionsURL       string
	PublishedRelative string
	PublishedTitle    string
}

// renderVersionPage renders web/version.html.tpl with the given rows into
// docs/version.html.
func renderVersionPage(rows []releaseRow, generatedAt time.Time) error {
	pageRows := make([]versionPageRow, 0, len(rows))
	stableReleasesSeen := 0
	for i, row := range rows {
		versionsDisplay := "No data"
		versionsURL := ""
		if len(row.K8sVersions) > 0 {
			versionsDisplay = strings.Join(row.K8sVersions, ", ")
			if row.WorkflowPath != "" {
				versionsURL = fmt.Sprintf("https://github.com/argoproj/argo-rollouts/blob/%s/%s#L%d", row.Tag, row.WorkflowPath, row.WorkflowLine)
			}
		}

		var supportStatus, supportClass string
		supportStatus, supportClass, stableReleasesSeen = supportTier(i, row.Tag, stableReleasesSeen)

		pageRows = append(pageRows, versionPageRow{
			Tag:               row.Tag,
			URL:               row.HTMLURL,
			SupportStatus:     supportStatus,
			SupportClass:      supportClass,
			VersionsDisplay:   versionsDisplay,
			VersionsURL:       versionsURL,
			PublishedRelative: humanize.RelTime(row.PublishedAt, generatedAt, "ago", "from now"),
			PublishedTitle:    row.PublishedAt.Format("02 Jan 2006"),
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

// supportTier classifies row index i (0 = master) by tag, given how many
// stable releases have been seen so far, returning the display status/class
// and the updated stableReleasesSeen counter.
func supportTier(i int, tag string, stableReleasesSeen int) (status, class string, newStableReleasesSeen int) {
	isRC := strings.Contains(strings.ToLower(tag), "rc")
	if i > 0 && !isRC {
		stableReleasesSeen++
		switch stableReleasesSeen {
		case 1:
			return "Supported", "diff-add", stableReleasesSeen
		case 2:
			return "Best-effort", "text-muted", stableReleasesSeen
		}
	}
	// master (i == 0), rc releases, and stable releases beyond the latest two fall through to Unsupported.
	return "Unsupported", "diff-del", stableReleasesSeen
}
