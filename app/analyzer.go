package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// analyze examines releases and generates alerts based on configured rules
func analyze(rc RepoConfig, releases []Release, supportWindowDays int) []Alert {
	var alerts []Alert
	pinned := normalize(rc.PinnedVersion)
	kwRegex := buildKeywordRegex(rc.CriticalKeywords)

	latest := releases[0]
	latestAge := time.Since(latest.PublishedAt).Hours() / 24

	// Keyword-based breaking change detection
	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		body := strings.ToLower(r.Body + " " + r.Name)
		if kwRegex != nil && kwRegex.MatchString(body) {
			alerts = append(alerts, Alert{
				Repo:        rc.Owner + "/" + rc.Repo,
				Type:        rc.Type,
				TagName:     r.TagName,
				Severity:    "critical",
				Reason:      "Breaking/deprecation language detected in release notes",
				PublishedAt: r.PublishedAt,
				URL:         r.HTMLURL,
				Remediation: remediationHint(rc.Type),
			})
		}
	}

	// Pinned version lag vs latest
	if normalize(latest.TagName) != pinned {
		sev := "warning"
		reason := "Pinned version behind latest"
		if latestAge >= float64(supportWindowDays) {
			sev = "critical"
			reason = fmt.Sprintf("Pinned version exceeds %d‑day support window", supportWindowDays)
		}
		alerts = append(alerts, Alert{
			Repo:        rc.Owner + "/" + rc.Repo,
			Type:        rc.Type,
			TagName:     latest.TagName,
			Severity:    sev,
			Reason:      reason,
			PublishedAt: latest.PublishedAt,
			URL:         latest.HTMLURL,
			Remediation: remediationHint(rc.Type),
		})
	}

	// Major version bump signal (helm/images often break on major version changes)
	if isMajorBump(pinned, normalize(latest.TagName)) {
		alerts = append(alerts, Alert{
			Repo:        rc.Owner + "/" + rc.Repo,
			Type:        rc.Type,
			TagName:     latest.TagName,
			Severity:    "critical",
			Reason:      "Detected major version bump; potential breaking changes",
			PublishedAt: latest.PublishedAt,
			URL:         latest.HTMLURL,
			Remediation: remediationHint(rc.Type),
		})
	}

	return alerts
}

// remediationHint provides context-specific remediation guidance
func remediationHint(t string) string {
	switch t {
	case "helm":
		return "🚨 UPGRADE ACTION REQUIRED: Review chart values and upgrade path; test in staging before rollout. Deprecated versions may lose support."
	case "image":
		return "🚨 UPGRADE ACTION REQUIRED: Rebuild runner images with new version; validate workflows against new tag; roll out via canary deployment. Old images may have security vulnerabilities."
	default:
		return "🚨 UPGRADE ACTION REQUIRED: Update runner binary to latest version; verify compatibility with ARC and workflows. Outdated binaries may fail to connect."
	}
}

// buildKeywordRegex creates a regex pattern from critical keywords
func buildKeywordRegex(keywords []string) *regexp.Regexp {
	if len(keywords) == 0 {
		return nil
	}
	var parts []string
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		k = regexp.QuoteMeta(strings.ToLower(k))
		parts = append(parts, k)
	}
	if len(parts) == 0 {
		return nil
	}
	pattern := "(" + strings.Join(parts, "|") + ")"
	return regexp.MustCompile(pattern)
}

// normalize standardizes version tag format
func normalize(tag string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(tag)), "v")
}

// isMajorBump detects if there's a major version change
func isMajorBump(oldTag, newTag string) bool {
	parse := func(s string) (int, int, int) {
		var M, m, p int
		fmt.Sscanf(s, "%d.%d.%d", &M, &m, &p)
		return M, m, p
	}
	oM, _, _ := parse(oldTag)
	nM, _, _ := parse(newTag)
	return oM != 0 && nM != 0 && nM > oM
}
