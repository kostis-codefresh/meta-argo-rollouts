# Argo Rollouts Meta

Automatic reporting for key statistics for [Argo Rollouts](https://github.com/argoproj/argo-rollouts).

This is a Go program that queries the GitHub API for the `argoproj/argo-rollouts` repository and generates a set of static HTML dashboard pages under `docs/`, published via GitHub Pages. It reports on:

- **Kubernetes version compatibility** — which k8s versions are tested in CI (`version.html`)
- **Ready-to-merge PRs** — open PRs with no conflicts, passing checks, and no pending review requests (`ready.html`)
- **Critical PRs** — ready PRs that also delete lines from existing `*_test.go` files, i.e. change behavior of existing tests (`critical.html`)
- **Slow tests** — e2e tests slower than 5 seconds in the latest completed run (`slow.html`)
- **Flaky tests** — tests that failed at least once across the last 10 sampled runs on master (`flaky.html`)

A GitHub Actions workflow (`.github/workflows/generate-report.yml`) regenerates these pages every 6 hours on weekdays and commits the result back to the repo.

## Running locally

The program authenticates to the GitHub API using a token from the `GH_TOKEN` environment variable. If you're already logged in with the [`gh` CLI](https://cli.github.com/), you can reuse that session's token:

```sh
GH_TOKEN=$(gh auth token) go run .
```

This fetches live data from GitHub, prints a summary to stdout, and (re)writes the HTML pages in `docs/`.
