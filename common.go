package main

import (
	"fmt"
	"io"
	"os"
)

// copyStaticAssets creates docs/img and copies the dashboard's shared CSS,
// favicon, and images from web/ into docs/, overwriting existing copies.
func copyStaticAssets() error {
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
	return nil
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
