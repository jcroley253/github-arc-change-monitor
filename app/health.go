package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RunnerVersionCheck represents a health check for runner version compatibility
type RunnerVersionCheck struct {
	Version    string
	IsHealthy  bool
	Message    string
	CheckedAt  time.Time
	StatusCode int
}

// checkRunnerHealth performs a health check to verify runner version compatibility
// This dynamically determines deprecation by fetching recent releases and applying heuristics
func checkRunnerHealth(ctx context.Context, client *http.Client, token, version string) (RunnerVersionCheck, error) {
	check := RunnerVersionCheck{
		Version:   version,
		CheckedAt: time.Now(),
	}

	// Normalize version for comparison
	normalizedVersion := strings.TrimPrefix(strings.TrimSpace(version), "v")

	// Parse the version numbers
	var major, minor, patch int
	fmt.Sscanf(normalizedVersion, "%d.%d.%d", &major, &minor, &patch)

	// Fetch recent releases to determine what's actively maintained
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/actions/runner/releases?per_page=50", nil)
	if err != nil {
		return check, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("🚨 Network error checking releases: %v", err)
		return check, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("🚨 Failed to fetch releases: HTTP %d", resp.StatusCode)
		return check, nil
	}

	body, _ := io.ReadAll(resp.Body)
	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("🚨 Failed to parse releases: %v", err)
		return check, nil
	}

	if len(releases) == 0 {
		check.IsHealthy = true
		check.Message = "⚠️ No releases found to compare against"
		check.StatusCode = 200
		return check, nil
	}

	// Get latest stable release
	var latestRelease *Release
	for i := range releases {
		if !releases[i].Prerelease && !releases[i].Draft {
			latestRelease = &releases[i]
			break
		}
	}

	if latestRelease == nil {
		check.IsHealthy = true
		check.Message = "⚠️ No stable releases found to compare against"
		check.StatusCode = 200
		return check, nil
	}

	latestVersion := strings.TrimPrefix(latestRelease.TagName, "v")
	var latestMajor, latestMinor, latestPatch int
	fmt.Sscanf(latestVersion, "%d.%d.%d", &latestMajor, &latestMinor, &latestPatch)

	// Calculate age of the pinned version by finding it in the release list
	var pinnedReleaseAge time.Duration
	var pinnedReleaseIndex int = -1
	pinnedVersionTag := "v" + normalizedVersion

	for i, rel := range releases {
		if strings.EqualFold(rel.TagName, pinnedVersionTag) ||
			strings.EqualFold(strings.TrimPrefix(rel.TagName, "v"), normalizedVersion) {
			pinnedReleaseAge = time.Since(rel.PublishedAt)
			pinnedReleaseIndex = i
			break
		}
	}

	// Rule 1: Major version behind = deprecated
	if major < latestMajor {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Runner image version %s is a major version behind latest %s (deprecated)", version, latestRelease.TagName)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 2: More than 4 minor versions behind = deprecated
	versionsBehind := latestMinor - minor
	if versionsBehind >= 5 {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Runner image version %s is %d minor versions behind latest %s (deprecated)", version, versionsBehind, latestRelease.TagName)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 3: Version older than 60 days AND 2+ minor versions behind = deprecated
	// GitHub typically deprecates versions after 60-90 days
	if versionsBehind >= 2 && pinnedReleaseAge > 60*24*time.Hour {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Runner image version %s is %d minor versions behind and %.0f days old (deprecated)", version, versionsBehind, pinnedReleaseAge.Hours()/24)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 4: 3-4 minor versions behind = critical (high risk)
	if versionsBehind >= 3 {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Runner image version %s is %d minor versions behind latest %s (high risk of deprecation)", version, versionsBehind, latestRelease.TagName)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 5: Behind but within acceptable range
	if versionsBehind > 0 || pinnedReleaseIndex > 0 {
		check.IsHealthy = true
		check.Message = fmt.Sprintf("Runner image version %s is behind latest %s but likely still supported (%d releases, %.0f days old)",
			version, latestRelease.TagName, pinnedReleaseIndex, pinnedReleaseAge.Hours()/24)
		check.StatusCode = 200
		return check, nil
	}

	// Version is current or not found in releases (assume healthy)
	check.IsHealthy = true
	check.Message = "\033[32m✅ Runner image version appears healthy\033[0m"
	check.StatusCode = 200
	return check, nil
}

// checkHelmChartHealth performs a health check for Helm chart versions
// Uses similar heuristics as runner version checks but adapted for chart versioning
func checkHelmChartHealth(ctx context.Context, client *http.Client, token, owner, repo, version string) (RunnerVersionCheck, error) {
	check := RunnerVersionCheck{
		Version:   version,
		CheckedAt: time.Now(),
	}

	// Normalize version for comparison
	normalizedVersion := strings.TrimPrefix(strings.TrimSpace(version), "v")

	// Parse the version numbers
	var major, minor, patch int
	fmt.Sscanf(normalizedVersion, "%d.%d.%d", &major, &minor, &patch)

	// Fetch recent releases
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=30", owner, repo), nil)
	if err != nil {
		return check, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("🚨 Network error checking releases: %v", err)
		return check, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("🚨 Failed to fetch releases: HTTP %d", resp.StatusCode)
		return check, nil
	}

	body, _ := io.ReadAll(resp.Body)
	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("🚨 Failed to parse releases: %v", err)
		return check, nil
	}

	if len(releases) == 0 {
		check.IsHealthy = true
		check.Message = "⚠️ No releases found to compare against"
		check.StatusCode = 200
		return check, nil
	}

	// Get latest stable release
	var latestRelease *Release
	for i := range releases {
		if !releases[i].Prerelease && !releases[i].Draft {
			latestRelease = &releases[i]
			break
		}
	}

	if latestRelease == nil {
		check.IsHealthy = true
		check.Message = "⚠️ No stable releases found to compare against"
		check.StatusCode = 200
		return check, nil
	}

	latestVersion := strings.TrimPrefix(latestRelease.TagName, "v")
	var latestMajor, latestMinor, latestPatch int
	fmt.Sscanf(latestVersion, "%d.%d.%d", &latestMajor, &latestMinor, &latestPatch)

	// Find the pinned version in releases
	var pinnedReleaseAge time.Duration
	var pinnedReleaseIndex int = -1
	pinnedVersionTag := "v" + normalizedVersion

	for i, rel := range releases {
		if strings.EqualFold(rel.TagName, pinnedVersionTag) ||
			strings.EqualFold(strings.TrimPrefix(rel.TagName, "v"), normalizedVersion) {
			pinnedReleaseAge = time.Since(rel.PublishedAt)
			pinnedReleaseIndex = i
			break
		}
	}

	// Helm chart specific rules (charts update less frequently than binaries)

	// Rule 1: Major version behind = critical (breaking changes in Helm charts)
	if major < latestMajor {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Helm chart version %s is a major version behind latest %s (likely breaking changes)", version, latestRelease.TagName)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 2: More than 3 minor versions behind = deprecated
	versionsBehind := latestMinor - minor
	if versionsBehind >= 4 {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Helm chart version %s is %d minor versions behind latest %s (deprecated)", version, versionsBehind, latestRelease.TagName)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 3: Version older than 90 days AND 2+ minor versions behind = deprecated
	if versionsBehind >= 2 && pinnedReleaseAge > 90*24*time.Hour {
		check.IsHealthy = false
		check.Message = fmt.Sprintf("Helm chart version %s is %d minor versions behind and %.0f days old (deprecated)", version, versionsBehind, pinnedReleaseAge.Hours()/24)
		check.StatusCode = 403
		return check, nil
	}

	// Rule 4: Behind but within acceptable range
	if versionsBehind > 0 || pinnedReleaseIndex > 0 {
		check.IsHealthy = true
		check.Message = fmt.Sprintf("Helm chart version %s is behind latest %s but likely still supported (%d releases, %.0f days old)",
			version, latestRelease.TagName, pinnedReleaseIndex, pinnedReleaseAge.Hours()/24)
		check.StatusCode = 200
		return check, nil
	}

	// Version is current
	check.IsHealthy = true
	check.Message = "\033[32m✅ Helm chart version appears healthy\033[0m"
	check.StatusCode = 200
	return check, nil
}

// analyzeRunnerHealth generates alerts for unhealthy runner versions
func analyzeRunnerHealth(checks []RunnerVersionCheck) []Alert {
	var alerts []Alert

	for _, check := range checks {
		if !check.IsHealthy {
			severity := "critical"
			if strings.Contains(strings.ToLower(check.Message), "deprecat") {
				severity = "critical"
			} else if check.StatusCode == http.StatusForbidden {
				severity = "critical"
			} else {
				severity = "warning"
			}

			alerts = append(alerts, Alert{
				Repo:        "version-health-check",
				Type:        "health",
				TagName:     check.Version,
				Severity:    severity,
				Reason:      fmt.Sprintf("⚠️ Version health check detected an issue: %s", check.Message),
				PublishedAt: check.CheckedAt,
				URL:         "https://github.com/actions/runner/releases",
				Remediation: "🚨 Update to the latest version immediately! Current version may be deprecated or incompatible.",
			})
		}
	}

	return alerts
}
