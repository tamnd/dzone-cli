package dzone_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/dzone-cli/dzone"
)

// rssXML wraps items in a minimal RSS 2.0 envelope.
func rssXML(items string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
` + items + `
  </channel>
</rss>`
}

func singleItem(title, link, pubDate, creator, category, description string) string {
	return `<item>
  <title>` + title + `</title>
  <link>` + link + `</link>
  <pubDate>` + pubDate + `</pubDate>
  <dc:creator><![CDATA[` + creator + `]]></dc:creator>
  <category><![CDATA[` + category + `]]></category>
  <description><![CDATA[` + description + `]]></description>
</item>`
}

func newTestClient(ts *httptest.Server) *dzone.Client {
	cfg := dzone.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return dzone.NewClient(cfg)
}

func TestLatestParsesTitle(t *testing.T) {
	xml := rssXML(singleItem(
		"Understanding Kubernetes Networking",
		"https://dzone.com/articles/kubernetes-networking",
		"Mon, 15 Jan 2024 12:00:00 GMT",
		"Jane Smith",
		"Kubernetes",
		"<p>A short summary about k8s.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if arts[0].Title != "Understanding Kubernetes Networking" {
		t.Errorf("Title = %q", arts[0].Title)
	}
}

func TestLatestParsesAuthor(t *testing.T) {
	xml := rssXML(singleItem(
		"Java 21 Features",
		"https://dzone.com/articles/java-21",
		"Wed, 10 Jan 2024 09:00:00 GMT",
		"John Doe",
		"Java",
		"<p>Body text.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].Author != "John Doe" {
		t.Errorf("Author = %q", arts[0].Author)
	}
}

func TestLatestParsesURL(t *testing.T) {
	wantURL := "https://dzone.com/articles/devops-pipeline-guide"
	xml := rssXML(singleItem(
		"DevOps Pipeline Guide",
		wantURL,
		"Fri, 12 Jan 2024 15:30:00 GMT",
		"Alice Green",
		"DevOps",
		"<p>Summary here.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].URL != wantURL {
		t.Errorf("URL = %q, want %q", arts[0].URL, wantURL)
	}
}

func TestLatestParsesDate(t *testing.T) {
	xml := rssXML(singleItem(
		"Security Flaw Found",
		"https://dzone.com/articles/security-flaw",
		"Thu, 07 Mar 2024 18:00:00 GMT",
		"Bob Security",
		"Security",
		"<p>Details.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if arts[0].Published != "2024-03-07" {
		t.Errorf("Published = %q, want %q", arts[0].Published, "2024-03-07")
	}
}

func TestLatestStripsSummaryHTML(t *testing.T) {
	xml := rssXML(singleItem(
		"Cloud Native Update",
		"https://dzone.com/articles/cloud-native",
		"Sat, 20 Jan 2024 10:00:00 GMT",
		"Carol Dev",
		"Cloud",
		"<p>This is the <b>summary</b> text.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(arts[0].Summary, "<") || strings.Contains(arts[0].Summary, ">") {
		t.Errorf("Summary contains HTML tags: %q", arts[0].Summary)
	}
	if !strings.Contains(arts[0].Summary, "summary") {
		t.Errorf("Summary text missing: %q", arts[0].Summary)
	}
}

func TestLatestTruncatesSummary(t *testing.T) {
	long := strings.Repeat("x", 300)
	xml := rssXML(singleItem(
		"Long Article",
		"https://dzone.com/articles/long",
		"Mon, 01 Jan 2024 00:00:00 GMT",
		"Author Name",
		"Java",
		"<p>"+long+"</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(arts[0].Summary)
	if len(runes) > 150 {
		t.Errorf("Summary too long: %d runes", len(runes))
	}
	if !strings.HasSuffix(arts[0].Summary, "…") {
		t.Errorf("Summary missing ellipsis: %q", arts[0].Summary)
	}
}

func TestLatestRankOrder(t *testing.T) {
	items := singleItem("A", "https://dzone.com/articles/a", "Mon, 01 Jan 2024 00:00:00 GMT", "X", "Java", "") +
		singleItem("B", "https://dzone.com/articles/b", "Tue, 02 Jan 2024 00:00:00 GMT", "Y", "Java", "") +
		singleItem("C", "https://dzone.com/articles/c", "Wed, 03 Jan 2024 00:00:00 GMT", "Z", "Java", "")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 3 {
		t.Fatalf("got %d articles, want 3", len(arts))
	}
	for i, a := range arts {
		if a.Rank != i+1 {
			t.Errorf("arts[%d].Rank = %d, want %d", i, a.Rank, i+1)
		}
	}
}

func TestLatestLimit(t *testing.T) {
	items := ""
	for i := 0; i < 5; i++ {
		items += singleItem("T", "https://dzone.com/articles/x", "Mon, 01 Jan 2024 00:00:00 GMT", "A", "Java", "")
	}
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Latest(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("got %d articles with limit=2, want 2", len(arts))
	}
}

func TestFeedUnknownSection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := newTestClient(ts).Feed(context.Background(), "nonexistent", 0)
	if !errors.Is(err, dzone.ErrUnknownSection) {
		t.Errorf("got %v, want ErrUnknownSection", err)
	}
}

func TestFeedKnownSection(t *testing.T) {
	xml := rssXML(singleItem(
		"Java Virtual Threads Deep Dive",
		"https://dzone.com/articles/java-virtual-threads",
		"Fri, 05 Jan 2024 08:00:00 GMT",
		"Java Author",
		"Java",
		"<p>Java summary.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Feed(context.Background(), "java", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 1 {
		t.Fatalf("got %d articles, want 1", len(arts))
	}
	if arts[0].Title != "Java Virtual Threads Deep Dive" {
		t.Errorf("Title = %q", arts[0].Title)
	}
}

func TestSearchFiltersResults(t *testing.T) {
	items := singleItem("Kubernetes Networking", "https://dzone.com/articles/k8s", "Mon, 01 Jan 2024 00:00:00 GMT", "A", "Kubernetes", "<p>k8s stuff</p>") +
		singleItem("Java Spring Boot", "https://dzone.com/articles/spring", "Tue, 02 Jan 2024 00:00:00 GMT", "B", "Java", "<p>spring stuff</p>") +
		singleItem("Kubernetes Security", "https://dzone.com/articles/k8s-sec", "Wed, 03 Jan 2024 00:00:00 GMT", "C", "Security", "<p>k8s security</p>")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Search(context.Background(), "kubernetes", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("got %d articles for 'kubernetes', want 2", len(arts))
	}
}

func TestSearchLimit(t *testing.T) {
	items := ""
	for i := 0; i < 5; i++ {
		items += singleItem("Java Article", "https://dzone.com/articles/java", "Mon, 01 Jan 2024 00:00:00 GMT", "A", "Java", "<p>java content</p>")
	}
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	arts, err := newTestClient(ts).Search(context.Background(), "java", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("got %d articles with limit=2, want 2", len(arts))
	}
}

func TestSectionsReturnsAll(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	secs := newTestClient(ts).Sections()
	if len(secs) == 0 {
		t.Fatal("Sections returned empty list")
	}
	for i, s := range secs {
		if s.Rank != i+1 {
			t.Errorf("secs[%d].Rank = %d, want %d", i, s.Rank, i+1)
		}
		if s.Name == "" {
			t.Errorf("secs[%d].Name is empty", i)
		}
		if !strings.HasPrefix(s.URL, ts.URL) {
			t.Errorf("secs[%d].URL = %q, should start with test server URL", i, s.URL)
		}
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	cfg := dzone.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := dzone.NewClient(cfg)

	start := time.Now()
	_, err := c.Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	cfg := dzone.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	c := dzone.NewClient(cfg)
	_, _ = c.Latest(context.Background(), 0)

	if gotUA == "" {
		t.Error("request carried no User-Agent")
	}
}
