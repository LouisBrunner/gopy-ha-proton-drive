package proton

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	proton_api_bridge "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/go-proton-api"
)

const (
	archiveSuffix  = ".tar"
	chunkSuffix    = ".tar.part"
	metadataSuffix = ".metadata.json"
)

var (
	errFileNotFound = errors.New("file not found")
)

func makeChunkSuffix(index uint32) string {
	return fmt.Sprintf(".%02d%s", index, chunkSuffix)
}

func radixFromArchive(name string) (string, error) {
	radix, found := strings.CutSuffix(name, archiveSuffix)
	if !found {
		return "", fmt.Errorf("invalid archive name %q", name)
	}
	return radix, nil
}

func radixFromMetadata(name string) (string, error) {
	radix, found := strings.CutSuffix(name, metadataSuffix)
	if !found {
		return "", fmt.Errorf("invalid metadata name %q", name)
	}
	return radix, nil
}

func extendedSuffix(instanceID, backupID, suffix string) string {
	return fmt.Sprintf("%s-%s%s", backupID, instanceID, suffix)
}

func makeFileName(base, suffix string) string {
	return fmt.Sprintf("%s%s", base, suffix)
}

func (me *Client) findFileIn(ctx context.Context, parentLinkID, fileName string) (*proton.Link, error) {
	file, err := me.findFileInFn(ctx, parentLinkID, func(file *proton_api_bridge.ProtonDirectoryData) bool {
		return file.Name == fileName
	})
	if err != nil {
		return nil, err
	}
	return file.Link, nil
}

func (me *Client) findFileInFn(ctx context.Context, parentLinkID string, predicate func(file *proton_api_bridge.ProtonDirectoryData) bool) (*proton_api_bridge.ProtonDirectoryData, error) {
	files, err := me.listFiles(ctx, parentLinkID)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if predicate(file) {
			return file, nil
		}
	}

	return nil, errFileNotFound
}

func (me *Client) listFiles(ctx context.Context, parentLinkID string) ([]*proton_api_bridge.ProtonDirectoryData, error) {
	files, err := me.drive.ListDirectory(ctx, parentLinkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}
	return files, nil
}

func (me *Client) readFile(ctx context.Context, link *proton.Link, details string) (string, error) {
	reader, _, _, err := me.drive.DownloadFile(ctx, link, 0)
	if err != nil {
		return "", fmt.Errorf("failed to download%s: %w", details, err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read content%s: %w", details, err)
	}
	return string(content), nil
}

func (me *Client) readMetadata(ctx context.Context, file *proton.Link) (*Metadata, string, error) {
	rawMetadata, err := me.readFile(ctx, file, " metadata")
	if err != nil {
		return nil, "", err
	}

	metadata, isNewFormat, err := deserializeMetadata([]byte(rawMetadata))
	if err != nil {
		return nil, rawMetadata, fmt.Errorf("failed to deserialize metadata: %w", err)
	}
	if !isNewFormat {
		return nil, rawMetadata, nil
	}
	return metadata, string(metadata.HAMetadataJSON), nil
}

func (me *Client) downloadFile(ctx context.Context, parentLinkID, filename string) (io.ReadCloser, error) {
	file, err := me.findFileIn(ctx, parentLinkID, filename)
	if err != nil {
		return nil, fmt.Errorf("could not find file %q: %w", filename, err)
	}

	reader, _, _, err := me.drive.DownloadFile(ctx, file, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to download file %q: %w", filename, err)
	}

	return reader, nil
}
