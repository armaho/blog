package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type articleInfo struct {
	ReleaseDate   string `json:"release_date"`
	WordCount     int    `json:"word_count"`
	EstimatedTime int    `json:"estimated_time"`
}

type article struct {
	date time.Time
	content string
	url string
}

var articles []article

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

	if after, ok := strings.CutPrefix(path, contentDir); ok {
		return targetDir + after
	}
	return path
}

func handleDirectory(path string) error {
	return createDir(targetPathFromContentPath(path))
}

func getArticleMetadata(path string) (articleInfo, error) {
	var metadata articleInfo

	metadataPath := filepath.Join(path, "metadata.json")
	file, err := os.Open(metadataPath)
	if err != nil {
		return metadata, fmt.Errorf("Cannot read metadata file for article: %s",
			metadataPath)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return metadata, fmt.Errorf("Cannot decode metadata: %s", err)
	}

	return metadata, nil
}

func addMetadataToArticle(metadata articleInfo, html string) string {
	metadataText := fmt.Sprintf("%s • %d words • %d minutes",
		metadata.ReleaseDate,
		metadata.WordCount,
		metadata.EstimatedTime)
	metadataTag := "<div class=\"article-info\"><p>" + metadataText + "</p></div>"
	return metadataTag + html
}

func convertArticlePathToUrl(path string) string {
	const marker = "/articles/"
	i := strings.Index(path, marker)
	if i == -1 {
		return path
	}

	return path[i:]
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

	if strings.Contains(path, "articles") && filepath.Base(path) == "index.html" {
		metadata, err := getArticleMetadata(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("Cannot add metadata: %s", err)
		}

		html = addMetadataToArticle(metadata, html)

		releaseDate, err := time.Parse("2006-01-02", metadata.ReleaseDate)
		if err != nil {
			return fmt.Errorf("Invalid date found in %s: %s", path, metadata.ReleaseDate)
		}
		art := article{
			content: html,
			url: convertArticlePathToUrl(path),
			date: releaseDate,
		}
		articles = append(articles, art)
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
	src, err := os.Open(path)
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
		return handleDirectory(path)
	}

	if filepath.Ext(path) == ".html" {
		return handleHtmlFile(path)
	}
	return handleNormalFile(path)
}

func generateHomePage() error {
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].date.After(articles[j].date)
	})

	var previews strings.Builder
	for _, a := range articles {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(a.content))
		if err != nil {
			panic(err)
		}

		doc.Find("h1").Each(func(i int, h1 *goquery.Selection) {
			titleText := h1.Text() 
			h1.SetHtml(fmt.Sprintf(`<a class="article-title-link" href="%s">%s</a>`, a.url, titleText))
		})

		modifiedHTML, err := doc.Html()
		if err != nil {
			panic(err)
		}

		previews.WriteString(fmt.Sprintf(
			`<div class="article-preview">%s</div>`,
			modifiedHTML,
		))
		previews.WriteString("\n")	
	}

	tmplFile, err := os.Open(templatePath())
	if err != nil {
		return err
	}
	defer tmplFile.Close()

	tmpl, err := goquery.NewDocumentFromReader(tmplFile)
	if err != nil {
		return fmt.Errorf("Failed to parse template: %w", err)
	}

	tmpl.Find("#content").SetHtml(previews.String())

	final, err := tmpl.Html()
	if err != nil {
		return fmt.Errorf("Failed to serialize HTML: %w", err)
	}

	err = os.WriteFile(filepath.Join(targetDirectory(), "index.html"), []byte(final), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write output: %w", err)
	}

	return nil
}

func main() {
	if err := deleteDirIfExists(targetDirectory()); err != nil {
		panic(err)
	}

	if err := filepath.WalkDir(contentDirectory(), contentFileHandler); err != nil {
		panic(err)
	}

	if err := generateHomePage(); err != nil {
		panic(err)
	}
}

