package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

func main() {
	storageAccountURL := "https://mystorageaccount.blob.core.windows.net/"

	// This automatically reads the AZURE_TENANT_ID, AZURE_CLIENT_ID, and AZURE_CLIENT_SECRET
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("Failed to obtain a credential: %v", err)
	}

	blobClient, err := azblob.NewClient(storageAccountURL, cred, nil)
	if err != nil {
		log.Fatalf("Failed to create blob client: %v", err)
	}

	containerName := "myStorageContainer"
	blobName := "state.json"
	log.Printf("Successfully authenticated to %s", storageAccountURL)
	log.Printf("Ready to read/write: %s/%s", containerName, blobName)

	var cfgPath string
	var testVersion string
	var runnerBinaryVersion string
	var runnerHelmVersion string
	var runnerImageVersion string
	flag.StringVar(&cfgPath, "config", "config.yaml", "Path to config")
	flag.StringVar(&testVersion, "test-version", "", "Test a specific runner version (e.g., v2.328.0)")
	flag.StringVar(&runnerBinaryVersion, "runner-binary-version", "", "Override runner binary pinned_version (e.g., v2.330.0)")
	flag.StringVar(&runnerHelmVersion, "runner-helm-version", "", "Override helm chart pinned_version (e.g., actions-runner-controller-0.23.7)")
	flag.StringVar(&runnerImageVersion, "runner-image-version", "", "Override runner image pinned_version (e.g., ubuntu22/20241015)")
	flag.Parse()

	cfg, err := loadConfig(cfgPath)
	must(err)

	// Override pinned versions from command line flags
	if runnerBinaryVersion != "" || runnerHelmVersion != "" || runnerImageVersion != "" {
		for i := range cfg.Repos {
			if cfg.Repos[i].Type == "binary" && runnerBinaryVersion != "" {
				cfg.Repos[i].PinnedVersion = runnerBinaryVersion
				fmt.Printf("Overriding binary version to: %s\n", runnerBinaryVersion)
			}
			if cfg.Repos[i].Type == "helm" && runnerHelmVersion != "" {
				cfg.Repos[i].PinnedVersion = runnerHelmVersion
				fmt.Printf("Overriding helm version to: %s\n", runnerHelmVersion)
			}
			if cfg.Repos[i].Type == "image" && runnerImageVersion != "" {
				cfg.Repos[i].PinnedVersion = runnerImageVersion
				fmt.Printf("Overriding image version to: %s\n", runnerImageVersion)
			}
		}
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "\033[33mWarning: GITHUB_TOKEN not set, using unauthenticated API (60 req/hour limit)\033[0m")
	}

	state := loadStateFromBlob(context.Background(), blobClient, containerName, blobName)
	httpClient := &http.Client{Timeout: 15 * time.Second}

	// Test mode to check a specific version
	if testVersion != "" {
		fmt.Printf("Testing runner version: %s\n", testVersion)
		check, err := checkRunnerHealth(context.Background(), httpClient, token, testVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Health check error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Version: %s\n", check.Version)
		fmt.Printf("Healthy: %v\n", check.IsHealthy)
		fmt.Printf("Status Code: %d\n", check.StatusCode)

		if check.IsHealthy {
			fmt.Printf("Message: \033[32m%s\033[0m\n", check.Message) // Green
		} else {
			fmt.Printf("Message: \033[31m%s\033[0m\n", check.Message) // Red
		}

		fmt.Printf("Checked At: %s\n", check.CheckedAt.Format(time.RFC3339))
		if !check.IsHealthy {
			os.Exit(1)
		}
		return
	}

	hasChanges, newAlerts := runOnce(context.Background(), httpClient, token, cfg, state)

	// Write binary version change signal to GITHUB_OUTPUT for workflow consumption
	if githubOutputFile := os.Getenv("GITHUB_OUTPUT"); githubOutputFile != "" {
		for _, a := range newAlerts {
			if a.Type == "binary" {
				f, err := os.OpenFile(githubOutputFile, os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					fmt.Fprintf(f, "binary_version_changed=true\n")
					fmt.Fprintf(f, "new_binary_version=%s\n", a.TagName)
					f.Close()
					log.Printf("GITHUB_OUTPUT: binary_version_changed=true, new_binary_version=%s", a.TagName)
				} else {
					log.Printf("Warning: failed to write to GITHUB_OUTPUT: %v", err)
				}
				break
			}
		}
	}

	if hasChanges {
		saveStateToBlob(context.Background(), blobClient, containerName, blobName, state)
	} else {
		log.Println("No state changes detected (no new alerts or ETags). Skipping Azure Blob upload.")
	}
}

func runOnce(ctx context.Context, client *http.Client, token string, cfg Config, state *State) (bool, []Alert) {
	totalAlerts := 0
	stateChanged := false
	var allAlerts []Alert // Collect all alerts for batched notification

	// Check repository releases
	for _, rc := range cfg.Repos {
		repoKey := fmt.Sprintf("%s/%s", rc.Owner, rc.Repo)
		releases, etag, err := fetchReleases(ctx, client, token, rc.Owner, rc.Repo, state.ETags[repoKey])
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch %s: %v\n", repoKey, err)
			continue
		}
		if etag != "" {
			state.ETags[repoKey] = etag
		}
		if etag != "" && state.ETags[repoKey] != etag {
			state.ETags[repoKey] = etag
			stateChanged = true
		}
		if len(releases) == 0 {
			continue
		}

		alerts := analyze(rc, releases, cfg.SupportWindowDays)
		for _, a := range alerts {
			// dedupe by stable hash
			id := stableID(repoKey, a.TagName, a.Reason)
			if _, ok := state.Seen[repoKey][id]; ok {
				continue
			}
			allAlerts = append(allAlerts, a)
			if state.Seen[repoKey] == nil {
				state.Seen[repoKey] = map[string]struct{}{}
			}
			state.Seen[repoKey][id] = struct{}{}
			totalAlerts++
			stateChanged = true
		}
	}

	// Check GitHub Changelog (if enabled)
	if cfg.EnableChangelog {
		entries, err := fetchChangelog(ctx, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch changelog: %v\n", err)
		} else {
			keywords := cfg.ChangelogKeywords
			if len(keywords) == 0 {
				// Default keywords if not specified
				keywords = []string{"breaking change", "deprecat", "end of support"}
			}
			alerts := analyzeChangelog(entries, keywords)
			for _, a := range alerts {
				if _, ok := state.SeenChangelogIDs[a.TagName]; ok {
					continue
				}
				allAlerts = append(allAlerts, a)
				state.SeenChangelogIDs[a.TagName] = struct{}{}
				totalAlerts++
				stateChanged = true
			}
		}
	}

	// Health check current versions (if enabled)
	if cfg.EnableHealthChecks {
		fmt.Println("Running health checks on pinned versions...")
		var checks []RunnerVersionCheck
		for _, rc := range cfg.Repos {
			if rc.PinnedVersion != "" && (rc.Type == "binary" || rc.Type == "helm") {
				fmt.Printf("  Checking %s version: %s\n", rc.Type, rc.PinnedVersion)
				var check RunnerVersionCheck
				var err error

				switch rc.Type {
				case "binary":
					check, err = checkRunnerHealth(ctx, client, token, rc.PinnedVersion)
				case "helm":
					check, err = checkHelmChartHealth(ctx, client, token, rc.Owner, rc.Repo, rc.PinnedVersion)
				}

				if err != nil {
					fmt.Fprintf(os.Stderr, "health check %s: %v\n", rc.PinnedVersion, err)
					continue
				}
				fmt.Printf("    Status: %v - %s\n", check.IsHealthy, check.Message)
				checks = append(checks, check)
			}
		}
		alerts := analyzeRunnerHealth(checks)
		for _, a := range alerts {
			repoKey := "version-health-check"
			id := stableID(repoKey, a.TagName, a.Reason)
			if state.Seen[repoKey] == nil {
				state.Seen[repoKey] = map[string]struct{}{}
			}
			if _, ok := state.Seen[repoKey][id]; ok {
				continue
			}
			allAlerts = append(allAlerts, a)
			state.Seen[repoKey][id] = struct{}{}
			totalAlerts++
			stateChanged = true
		}
	}

	// Send all alerts in a single notification
	if len(allAlerts) > 0 {
		notifyBatch(cfg.Notifications, allAlerts)
	}

	if totalAlerts == 0 {
		fmt.Println("\033[32mNo breaking changes that will affect GitHub Action Runners detected!\033[0m")
	}
	return stateChanged, allAlerts
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
