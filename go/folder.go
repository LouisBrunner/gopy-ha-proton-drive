package proton

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	proton_api_bridge "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/go-proton-api"
)

func (me *Client) MakeRootFolder(ctx context.Context, path string) (*Folder, error) {
	folders := strings.Split(path, "/")
	currentFolder := me.drive.RootLink
	for i, folder := range folders {
		if folder == "" {
			continue
		}
		pathSoFar := filepath.Join(folders[:i+1]...)

		nextFolder, err := me.findFileIn(ctx, currentFolder.LinkID, folder)
		if err != nil {
			if err != errFileNotFound {
				return nil, fmt.Errorf("failed to list directory %q: %w", pathSoFar, err)
			}
		}
		if nextFolder == nil {
			newFolder, err := me.drive.CreateNewFolder(ctx, currentFolder, folder)
			if err != nil {
				return nil, fmt.Errorf("failed to create folder %q in %q: %w", folder, pathSoFar, err)
			}
			nextFolder, err = me.drive.GetLink(ctx, newFolder)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve new folder %q in %q: %w", folder, pathSoFar, err)
			}
		}

		if nextFolder.Type != proton.LinkTypeFolder {
			return nil, fmt.Errorf("file %q is not a folder", pathSoFar)
		}

		currentFolder = nextFolder
	}
	return &Folder{
		Link:   currentFolder,
		client: me,
	}, nil
}

type Folder struct {
	*proton.Link
	client *Client
}

func (me *Folder) FindBackup(ctx context.Context, instanceID, backupID string) (string, error) {
	suffix := extendedSuffix(instanceID, backupID, archiveSuffix)
	file, err := me.client.findFileInFn(ctx, me.LinkID, func(file *proton_api_bridge.ProtonDirectoryData) bool {
		return strings.HasSuffix(file.Name, suffix)
	})
	if err != nil {
		return "", fmt.Errorf("failed to find metadata for %q: %w", backupID, err)
	}

	return file.Link.LinkID, nil
}

func (me *Folder) Upload(ctx context.Context, instanceID, backupID, name, metadataJSON, contentPath string) error {
	baseName, err := radixFromArchive(name)
	if err != nil {
		return err
	}
	suffix := extendedSuffix(instanceID, backupID, "")

	archiveName := fmt.Sprintf("%s%s%s", baseName, suffix, archiveSuffix)
	metadataName := fmt.Sprintf("%s%s%s", baseName, suffix, metadataSuffix)

	_, err = me.client.findFileIn(ctx, me.LinkID, archiveName)
	if err == nil {
		return fmt.Errorf("backup %q already exists", name)
	}

	_, err = me.client.findFileIn(ctx, me.LinkID, metadataName)
	if err == nil {
		return fmt.Errorf("metadata for backup %q already exists", name)
	}

	_, _, err = me.client.drive.UploadFileByPath(ctx, me.Link, archiveName, contentPath, 0)
	if err != nil {
		return fmt.Errorf("failed to upload backup %q: %w", name, err)
	}

	_, _, err = me.client.drive.UploadFileByReader(ctx, me.LinkID, metadataName, time.Now(), strings.NewReader(metadataJSON), 0)
	if err != nil {
		return fmt.Errorf("failed to upload metadata for backup %q: %w", name, err)
	}

	return nil
}

func (me *Folder) ListFilesMetadata(ctx context.Context, instanceID string) ([]string, error) {
	suffix := extendedSuffix(instanceID, "", metadataSuffix)

	files, err := me.client.drive.ListDirectory(ctx, me.LinkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	metadatas := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.HasSuffix(file.Name, suffix) {
			continue
		}

		metadata, err := me.client.readFile(ctx, file.Link, " metadata")
		if err != nil {
			return nil, err
		}

		metadatas = append(metadatas, metadata)
	}

	return metadatas, nil
}
