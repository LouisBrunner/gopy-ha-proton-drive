package proton

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-version"
)

var (
	MetadataLowestSupportedVersion = version.Must(version.NewVersion("1.0.0"))
	MetadataCurrentVersion         = version.Must(version.NewVersion("1.0.0"))
)

type MetadataHeader struct {
	Version *version.Version `json:"proton_drive_version"`
}

type Metadata struct {
	MetadataHeader
	InstanceID     string          `json:"instance_id"`
	BackupID       string          `json:"backup_id"`
	BaseName       string          `json:"base_name"`
	HAMetadataJSON json.RawMessage `json:"metadata"`
	Chunks         uint32          `json:"chunks"`
}

func deserializeMetadata(data []byte) (*Metadata, bool, error) {
	var header MetadataHeader
	err := json.Unmarshal(data, &header)
	if err != nil {
		return nil, false, err
	}
	if header.Version == nil {
		return nil, false, nil
	}
	if header.Version.LessThan(MetadataLowestSupportedVersion) {
		return nil, false, fmt.Errorf("unsupported metadata version %s, lowest supported is %s", header.Version, MetadataLowestSupportedVersion)
	}
	if header.Version.GreaterThan(MetadataCurrentVersion) {
		return nil, false, fmt.Errorf("unsupported metadata version %s, highest supported is %s", header.Version, MetadataCurrentVersion)
	}
	var metadata Metadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, false, err
	}
	return &metadata, true, nil
}
