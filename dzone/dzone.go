// Package dzone is the library behind the dz command: the HTTP client,
// request shaping, and the typed data models for DZone.
//
// Data comes from the public RSS feeds at feeds.dzone.com. No API key is
// required. The client sends a real User-Agent, paces requests, and retries
// 429/5xx with exponential backoff.
package dzone

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrUnknownSection is returned when the section argument does not match any
// registered feed key.
var ErrUnknownSection = errors.New("unknown section")

// feedEntry maps a section key to its feed path (relative to BaseURL).
type feedEntry struct {
	key   string
	path  string
	label string
}

// feeds lists all known DZone topic feeds.
var feeds = []feedEntry{
	{"home", "/home", "All recent articles"},
	{"java", "/java", "Java"},
	{"security", "/security", "Security"},
	{"microservices", "/microservices", "Microservices"},
	{"performance", "/performance", "Performance"},
	{"agile", "/agile", "Agile"},
	{"big-data", "/big-data", "Big Data"},
	{"open-source", "/open-source", "Open Source"},
	{"data", "/data", "Data"},
	{"iot", "/iot", "Internet of Things"},
	{"languages", "/languages", "Languages"},
	{"integration", "/integration", "Integration"},
}

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://feeds.dzone.com",
		UserAgent: "dz/dev (+https://github.com/tamnd/dzone-cli)",
		Rate:      300 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client fetches DZone RSS feeds.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client configured by cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// Article is the record emitted for each DZone article.
type Article struct {
	Rank      int    `json:"rank"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Published string `json:"published"`
	Summary   string `json:"summary"`
	Tags      string `json:"tags"`
	URL       string `json:"url"`
}

// Section is the record emitted by the sections command.
type Section struct {
	Rank  int    `json:"rank"`
	Name  string `json:"name"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Latest fetches the /home feed and returns up to limit articles ranked by
// feed order. limit=0 returns all entries.
func (c *Client) Latest(ctx context.Context, limit int) ([]Article, error) {
	return c.fetchFeed(ctx, c.baseURL+"/home", limit)
}

// Feed fetches the topic-specific feed for section and returns up to limit
// articles. Returns ErrUnknownSection if section is not registered.
func (c *Client) Feed(ctx context.Context, section string, limit int) ([]Article, error) {
	fe, ok := feedByKey(section)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownSection, section)
	}
	return c.fetchFeed(ctx, c.baseURL+fe.path, limit)
}

// Search fetches the /home feed and returns articles whose title or summary
// contains query (case-insensitive). At most limit results are returned;
// limit=0 returns all matches.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Article, error) {
	all, err := c.Latest(ctx, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Article
	for _, a := range all {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Summary), q) ||
			strings.Contains(strings.ToLower(a.Tags), q) {
			out = append(out, a)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// Sections returns the static list of registered feed sections.
// No network request is made.
func (c *Client) Sections() []Section {
	out := make([]Section, len(feeds))
	for i, fe := range feeds {
		out[i] = Section{
			Rank:  i + 1,
			Name:  fe.key,
			Label: fe.label,
			URL:   c.baseURL + fe.path,
		}
	}
	return out
}

// fetchFeed fetches the raw RSS at rawURL and converts it to Articles.
func (c *Client) fetchFeed(ctx context.Context, rawURL string, limit int) ([]Article, error) {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", rawURL, err)
	}
	items := feed.Channel.Items
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	out := make([]Article, len(items))
	for i, it := range items {
		out[i] = itemToArticle(it, i+1)
	}
	return out, nil
}

// feedByKey looks up a feed entry by its section key (case-insensitive).
func feedByKey(key string) (feedEntry, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, fe := range feeds {
		if fe.key == key {
			return fe, true
		}
	}
	return feedEntry{}, false
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/xml, application/rss+xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
