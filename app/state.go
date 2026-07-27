// State management for ETags and seen alerts. This will prevent daily emails for the same issues.
package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// State tracks ETags and seen alerts to avoid duplicates
type State struct {
	ETags            map[string]string              `json:"etags"`
	Seen             map[string]map[string]struct{} `json:"seen"` // repoKey -> tag -> {}
	SeenChangelogIDs map[string]struct{}            `json:"seen_changelog_ids"`
}

// Download state from Azure Blob
func loadStateFromBlob(ctx context.Context, client *azblob.Client, containerName, blobName string) *State {
	log.Printf("Attempting to download state from %s/%s...", containerName, blobName)

	resp, err := client.DownloadStream(ctx, containerName, blobName, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		// If the blob doesn't exist (404), this is the first run! Return an empty initialized state.
		if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
			log.Println("State blob not found. Initializing a fresh state.")
			return &State{
				Seen:             make(map[string]map[string]struct{}),
				SeenChangelogIDs: make(map[string]struct{}),
				ETags:            make(map[string]string),
			}
		}
		log.Fatalf("Fatal error downloading state blob: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read blob body: %v", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		log.Fatalf("Failed to parse state JSON: %v", err)
	}

	// Ensure maps are initialized if the JSON was empty
	if state.Seen == nil {
		state.Seen = make(map[string]map[string]struct{})
	}
	if state.SeenChangelogIDs == nil {
		state.SeenChangelogIDs = make(map[string]struct{})
	}
	if state.ETags == nil {
		state.ETags = make(map[string]string)
	}

	log.Println("State downloaded and parsed successfully.")
	return &state
}

// Upload state to Azure Blob
func saveStateToBlob(ctx context.Context, client *azblob.Client, containerName, blobName string, state *State) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Fatalf("Failed to serialize state to JSON: %v", err)
	}

	log.Printf("Uploading updated state to %s/%s...", containerName, blobName)

	// UploadBuffer automatically creates the file if it doesn't exist, or overwrites it if it does
	_, err = client.UploadBuffer(ctx, containerName, blobName, data, nil)
	if err != nil {
		log.Fatalf("Failed to upload state blob: %v", err)
	}

	log.Println("State successfully uploaded to Azure!")
}

// stableID generates a unique identifier for an alert
func stableID(repoKey, tag, reason string) string {
	h := sha1.New()
	io.WriteString(h, repoKey+"|"+tag+"|"+reason)
	return hex.EncodeToString(h.Sum(nil))
}
