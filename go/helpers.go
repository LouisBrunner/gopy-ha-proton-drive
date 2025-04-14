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
	metadataSuffix = ".metadata.json"
)

var (
	errFileNotFound = errors.New("file not found")
)

func radixFromArchive(name string) (string, error) {
	radix, found := strings.CutSuffix(name, ".tar")
	if !found {
		return "", fmt.Errorf("invalid archive name %q", name)
	}
	return radix, nil
}

func metadataFromArchive(name string) (string, error) {
	radix, err := radixFromArchive(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s", radix, metadataSuffix), nil
}

func extendedSuffix(instanceID, backupID, suffix string) string {
	return fmt.Sprintf("%s-%s%s", backupID, instanceID, suffix)
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
	files, err := me.drive.ListDirectory(ctx, parentLinkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}
	for _, file := range files {
		if predicate(file) {
			return file, nil
		}
	}

	return nil, errFileNotFound
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

func (me *Client) getName(ctx context.Context, file *proton.Link) (string, error) {
	fileData, err := me.findFileInFn(ctx, file.ParentLinkID, func(data *proton_api_bridge.ProtonDirectoryData) bool {
		return data.Link.LinkID == file.LinkID
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file name for %q: %w", file.LinkID, err)
	}
	return fileData.Name, nil
}
