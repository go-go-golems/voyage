package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func main() {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		markdown, err := os.ReadFile("./cmd/markdown/example.md")
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to read markdown file: %v", err))
			return
		}

		md := goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		)

		buf := &bytes.Buffer{}
		err = md.Convert(markdown, buf)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to convert markdown to HTML: %v", err))
			return
		}

		htmlContent := fmt.Sprintf(`
			<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Markdown Renderer</title>
				<link rel="stylesheet" href="/static/github-markdown-light.css">
			</head>
			<body>
				<article class="markdown-body">
					%s
				</article>
			</body>
			</html>
		`, buf)

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlContent))
	})

	router.Static("/static", "./cmd/markdown/static")

	router.Run(":8080")
}
