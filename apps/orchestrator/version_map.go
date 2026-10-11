package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"pkg/minioutil"

	"github.com/minio/minio-go/v7"
)

// A pinned, operator-produced map preserves old markers without rewriting audits.
// Only historical revert reads use it; new writes retain native destination IDs.
type mappedVersion struct {
	Key                string   `json:"key"`
	SourceVersion      string   `json:"sourceVersion"`
	Aliases            []string `json:"aliases,omitempty"`
	DestinationVersion string   `json:"destinationVersion"`
	DeleteMarker       bool     `json:"deleteMarker"`
	SHA256             string   `json:"sha256"`
	Size               int64    `json:"size"`
}

type versionMap struct {
	SchemaVersion       int             `json:"schemaVersion"`
	SourceEndpoint      string          `json:"sourceEndpoint"`
	DestinationEndpoint string          `json:"destinationEndpoint"`
	SourceBucket        string          `json:"sourceBucket"`
	DestinationBucket   string          `json:"destinationBucket"`
	Entries             []mappedVersion `json:"entries"`
	index               map[[2]string]mappedVersion
}

var historicalVersions *versionMap

func parseVersionMap(body []byte, endpoint, bucket string) (*versionMap, error) {
	var mapping versionMap
	if err := json.Unmarshal(body, &mapping); err != nil {
		return nil, fmt.Errorf("invalid S3 version map JSON: %w", err)
	}
	if mapping.SchemaVersion != 1 || mapping.SourceEndpoint == "" ||
		strings.TrimSuffix(mapping.DestinationEndpoint, "/") != strings.TrimSuffix(endpoint, "/") ||
		mapping.SourceBucket != bucket || mapping.DestinationBucket != bucket {
		return nil, fmt.Errorf("S3 version map schema, endpoint or bucket mismatch")
	}
	mapping.index = make(map[[2]string]mappedVersion, len(mapping.Entries))
	destinations := map[[2]string]bool{}
	for _, entry := range mapping.Entries {
		identity := [2]string{entry.Key, entry.SourceVersion}
		destination := [2]string{entry.Key, entry.DestinationVersion}
		_, duplicate := mapping.index[identity]
		checksum, err := hex.DecodeString(entry.SHA256)
		if entry.Key == "" || entry.SourceVersion == "" || entry.DestinationVersion == "" ||
			entry.DestinationVersion == "null" || duplicate || destinations[destination] || entry.Size < 0 ||
			(!entry.DeleteMarker && (err != nil || len(checksum) != sha256.Size)) {
			return nil, fmt.Errorf("invalid or duplicate S3 version map entry")
		}
		for _, alias := range append([]string{entry.SourceVersion}, entry.Aliases...) {
			aliasKey := [2]string{entry.Key, alias}
			if _, exists := mapping.index[aliasKey]; alias == "" || exists {
				return nil, fmt.Errorf("invalid or ambiguous historical version alias")
			}
			mapping.index[aliasKey] = entry
		}
		destinations[destination] = true
	}
	for identity := range mapping.index {
		if destinations[identity] {
			return nil, fmt.Errorf("ambiguous source/destination version ID collision")
		}
	}
	return &mapping, nil
}

func loadVersionMap(ctx context.Context, endpoint, bucket, key, version string) error {
	if key == "" && version == "" {
		return nil
	}
	if key == "" || version == "" || version == "null" {
		return fmt.Errorf("S3_VERSION_MAP_KEY and immutable S3_VERSION_MAP_VERSION are both required")
	}
	object, err := minioutil.GetObjectFromBucket(ctx, bucket, key, minio.GetObjectOptions{VersionID: version})
	if err != nil {
		return err
	}
	defer object.Close()
	const maxMapSize = 32 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(object, maxMapSize+1))
	if err != nil {
		return err
	}
	if len(body) > maxMapSize {
		return fmt.Errorf("S3 version map exceeds 32 MiB")
	}
	historicalVersions, err = parseVersionMap(body, endpoint, bucket)
	return err
}

func (mapping *versionMap) resolve(bucket, key, version string) (mappedVersion, bool, error) {
	if mapping == nil || bucket != mapping.SourceBucket {
		return mappedVersion{}, false, nil
	}
	entry, found := mapping.index[[2]string{key, version}]
	if found && entry.DeleteMarker {
		return mappedVersion{}, false, fmt.Errorf("requested historical version is a delete marker")
	}
	return entry, found, nil
}
