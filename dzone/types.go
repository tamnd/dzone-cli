package dzone

import (
	"encoding/xml"
	"strings"
	"time"
)

// ─── RSS 2.0 wire types ───────────────────────────────────────────────────────

// rssFeed is the root of an RSS 2.0 document from feeds.dzone.com.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

// rssItem maps to each <item> in the feed.
// dc:creator maps to the local name "creator" (encoding/xml matches local name).
type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PubDate     string   `xml:"pubDate"`
	Creator     string   `xml:"creator"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category"`
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// parseDate parses an RSS pubDate and returns "2006-01-02".
// Falls back to the raw string on parse error.
func parseDate(s string) string {
	s = strings.TrimSpace(s)
	// "Fri, 12 Jun 2026 19:00:00 GMT"
	for _, layout := range []string{time.RFC1123Z, time.RFC1123} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return s
}

// stripAndTruncate strips HTML tags, decodes common entities, and truncates to
// maxChars runes, appending "…" if truncated.
func stripAndTruncate(html string, maxChars int) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	out = strings.ReplaceAll(out, "&nbsp;", " ")
	out = strings.TrimSpace(out)
	rs := []rune(out)
	if len(rs) > maxChars {
		return string(rs[:maxChars-1]) + "…"
	}
	return out
}

// joinCategories lowercases and joins the categories with semicolons.
func joinCategories(cats []string) string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, strings.ToLower(c))
		}
	}
	return strings.Join(out, ";")
}

func itemToArticle(it rssItem, rank int) Article {
	return Article{
		Rank:      rank,
		Title:     strings.TrimSpace(it.Title),
		Author:    strings.TrimSpace(it.Creator),
		Published: parseDate(it.PubDate),
		Summary:   stripAndTruncate(it.Description, 150),
		Tags:      joinCategories(it.Categories),
		URL:       strings.TrimSpace(it.Link),
	}
}
