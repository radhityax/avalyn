package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

func rssFeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")

	rows, err := db.Query(`SELECT date, author, title, slug, content FROM posts 
		WHERE type='blog' AND status='post' 
		ORDER BY date DESC LIMIT 50`)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	type Item struct {
		Date    string
		Author  string
		Title   string
		Slug    string
		Content string
	}

	var items []Item
	for rows.Next() {
		var it Item
		rows.Scan(&it.Date, &it.Author, &it.Title, &it.Slug, &it.Content)
		items = append(items, it)
	}

	gm := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<rss version="2.0">`)
	sb.WriteString("<channel>")
	sb.WriteString(fmt.Sprintf("<title>%s</title>", escapeXML(site_title)))
	sb.WriteString(fmt.Sprintf("<link>%s</link>", escapeXML(site_url)))
	sb.WriteString(fmt.Sprintf("<description>%s</description>", escapeXML(site_subtitle)))
	sb.WriteString(fmt.Sprintf("<language>en-us</language>"))
	sb.WriteString(fmt.Sprintf("<lastBuildDate>%s</lastBuildDate>", time.Now().Format(time.RFC1123Z)))

	for _, it := range items {
		var contentHTML strings.Builder
		gm.Convert([]byte(it.Content), &contentHTML)

		sb.WriteString("<item>")
		sb.WriteString(fmt.Sprintf("<title>%s</title>", escapeXML(it.Title)))
		sb.WriteString(fmt.Sprintf("<link>%s/blog/%s</link>", escapeXML(site_url), it.Slug))
		sb.WriteString(fmt.Sprintf("<author>%s</author>", escapeXML(it.Author)))
		sb.WriteString(fmt.Sprintf("<pubDate>%s</pubDate>", formatDate(it.Date)))
		sb.WriteString(fmt.Sprintf("<description>%s</description>", escapeXML(contentHTML.String())))
		sb.WriteString(fmt.Sprintf("<guid>%s/blog/%s</guid>", escapeXML(site_url), it.Slug))
		sb.WriteString("</item>")
	}

	sb.WriteString("</channel>")
	sb.WriteString("</rss>")

	w.Write([]byte(sb.String()))
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func formatDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format(time.RFC1123Z)
}
