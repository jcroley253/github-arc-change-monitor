package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RSS feed structures for GitHub Changelog
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// ChangelogEntry represents a parsed changelog item
type ChangelogEntry struct {
	Title       string
	Link        string
	Description string
	PublishedAt time.Time
	GUID        string
}

// fetchChangelog retrieves GitHub changelog RSS feed
func fetchChangelog(ctx context.Context, client *http.Client) ([]ChangelogEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://github.blog/changelog/label/actions/feed/", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var rss RSS
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&rss); err != nil {
		return nil, err
	}

	var entries []ChangelogEntry
	for _, item := range rss.Channel.Items {
		pubTime, _ := time.Parse(time.RFC1123Z, item.PubDate)
		entries = append(entries, ChangelogEntry{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			PublishedAt: pubTime,
			GUID:        item.GUID,
		})
	}

	return entries, nil
}

// analyzeChangelog scans changelog entries for runner-related deprecations
func analyzeChangelog(entries []ChangelogEntry, keywords []string) []Alert {
	var alerts []Alert
	kwRegex := buildKeywordRegex(keywords)

	// Keywords specific to runner deprecations
	deprecationKeywords := []string{
		"runner version",
		"deprecated",
		"deprecation",
		"end of support",
		"no longer supported",
		"removed support",
		"runner update required",
	}
	deprecationRegex := buildKeywordRegex(deprecationKeywords)

	for _, entry := range entries {
		text := strings.ToLower(entry.Title + " " + entry.Description)

		// Check if it's runner-related
		if !strings.Contains(text, "runner") && !strings.Contains(text, "actions") {
			continue
		}

		severity := "info"
		reason := "GitHub Actions changelog update"

		// Check for critical keywords
		if kwRegex != nil && kwRegex.MatchString(text) {
			severity = "critical"
			reason = "Breaking/deprecation detected in GitHub changelog"
		} else if deprecationRegex != nil && deprecationRegex.MatchString(text) {
			severity = "critical"
			reason = "Runner version deprecation detected in changelog"
		}

		// Only alert on critical items to reduce noise
		if severity == "critical" {
			alerts = append(alerts, Alert{
				Repo:        "github/changelog",
				Type:        "changelog",
				TagName:     entry.GUID,
				Severity:    severity,
				Reason:      reason,
				PublishedAt: entry.PublishedAt,
				URL:         entry.Link,
				Remediation: fmt.Sprintf("Review changelog: %s - %s", entry.Title, entry.Link),
			})
		}
	}

	return alerts
}
