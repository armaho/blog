package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func createDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

func deleteDirIfExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove directory %s: %w", path, err)
		}
	}
	
	return nil
}

func contentDirectory() string {
	path := os.Getenv("CONTENT_PATH")
	if path == "" {
		panic("CONTENT_PATH is not set")
	}

	return path
}

func targetDirectory() string {
	path := os.Getenv("TARGET_PATH")
	if path == "" {
		panic("TARGET_PATH is not set")
	}

	return path
}

func templatePath() string {
	path := os.Getenv("TEMPLATE_PATH")
	if path == "" {
		panic("TEMPLATE_PATH is not set")
	}

	return path
}

func targetPathFromContentPath(path string) string {
	targetDir := targetDirectory()
	contentDir := contentDirectory()

	if after, ok := strings.CutPrefix(path, contentDir); ok  {
		return targetDir + after
	}
	return path;
}

func handleDirectory(path string) error {
	return createDir(targetPathFromContentPath(path));
}

func handleHtmlFile(path string) error {
	tmplFile, err := os.Open(templatePath())
	if err != nil {
		return fmt.Errorf("Failed to open template: %w", err)
	}
	defer tmplFile.Close()

	tmplDoc, err := goquery.NewDocumentFromReader(tmplFile)
	if err != nil {
		return fmt.Errorf("Failed to parse template: %w", err)
	}

	// Load source HTML
	srcFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Failed to open source: %w", err)
	}
	defer srcFile.Close()

	srcDoc, err := goquery.NewDocumentFromReader(srcFile)
	if err != nil {
		return fmt.Errorf("Failed to parse source: %w", err)
	}

	html, err := srcDoc.Find("body").Html()
	if err != nil || html == "" {
		html, err = srcDoc.Html()
		if err != nil {
			return fmt.Errorf("Failed to extract HTML: %w", err)
		}
	}

	tmplDoc.Find("#content").SetHtml(html)

	final, err := tmplDoc.Html()
	if err != nil {
		return fmt.Errorf("Failed to serialize HTML: %w", err)
	}

	err = os.WriteFile(targetPathFromContentPath(path), []byte(final), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write output: %w", err)
	}

	return nil
}

func handleNormalFile(path string) error {
	src, err := os.Open(path);
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(targetPathFromContentPath(path))
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func contentFileHandler(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		panic(fmt.Sprintf("Error walking content directory: %s", err))
	}

	if entry.IsDir() {
		return handleDirectory(path);
	}

	if filepath.Ext(path) == ".html" {
		return handleHtmlFile(path)
	}
	return handleNormalFile(path)
}

func main() {
	if err := deleteDirIfExists(targetDirectory()); err != nil {
		panic(err)
	}

	if err := filepath.WalkDir(contentDirectory(), contentFileHandler); err != nil {
		panic(err)
	}
}

