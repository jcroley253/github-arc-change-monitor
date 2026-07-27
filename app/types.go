package main

import "time"

// Alert represents a notification about a repository release
type Alert struct {
	Repo        string
	Type        string
	TagName     string
	Severity    string // critical|warning|info
	Reason      string
	PublishedAt time.Time
	URL         string
	Remediation string
}
