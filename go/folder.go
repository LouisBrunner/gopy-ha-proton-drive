package proton

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	proton_api_bridge "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/go-proton-api"
)

const (
	chunkMaxSize = 2 * 1024 * 1024 * 1024 // 2 GB
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

func (me *Folder) Upload(ctx context.Context, instanceID, backupID, name, haMetadataJSON, contentPath string) error {
	baseName, err := radixFromArchive(name)
	if err != nil {
		return err
	}

	archiveName := makeFileName(baseName, extendedSuffix(instanceID, backupID, archiveSuffix))
	metadataName := makeFileName(baseName, extendedSuffix(instanceID, backupID, metadataSuffix))
	firstChunkName := makeFileName(baseName, extendedSuffix(instanceID, backupID, makeChunkSuffix(0)))

	for label, checkName := range map[string]string{
		"archive":     archiveName,
		"metadata":    metadataName,
		"first chunk": firstChunkName,
	} {
		_, err = me.client.findFileIn(ctx, me.LinkID, checkName)
		if err == nil {
			return fmt.Errorf("%s %q for backup %q already exists", label, checkName, name)
		}
	}

	fstat, err := os.Stat(contentPath)
	if err != nil {
		return fmt.Errorf("failed to stat content file %q: %w", contentPath, err)
	}

	fileSize := fstat.Size()
	chunks := uint32(fileSize) / chunkMaxSize
	if fileSize%chunkMaxSize != 0 {
		chunks += 1
	}

	metadataJSON, err := json.Marshal(Metadata{
		MetadataHeader: MetadataHeader{
			Version: MetadataCurrentVersion,
		},
		InstanceID:     instanceID,
		BackupID:       backupID,
		BaseName:       baseName,
		HAMetadataJSON: json.RawMessage(haMetadataJSON),
		Chunks:         chunks,
	})
	if err != nil {
		return fmt.Errorf("failed to serialize metadata for backup %q: %w", name, err)
	}

	_, _, err = me.client.drive.UploadFileByReader(ctx, me.LinkID, metadataName, time.Now(), strings.NewReader(string(metadataJSON)), 0)
	if err != nil {
		return fmt.Errorf("failed to upload metadata for backup %q: %w", name, err)
	}

	if chunks > 1 {
		file, err := os.Open(contentPath)
		if err != nil {
			return fmt.Errorf("failed to open content file %q: %w", contentPath, err)
		}
		defer file.Close()

		for i := uint32(0); i < chunks; i += 1 {
			chunkName := makeFileName(baseName, extendedSuffix(instanceID, backupID, makeChunkSuffix(i)))
			reader := io.NewSectionReader(file, int64(i)*chunkMaxSize, int64(chunkMaxSize))
			_, _, err = me.client.drive.UploadFileByReader(ctx, me.LinkID, chunkName, time.Now(), reader, 0)
			if err != nil {
				return fmt.Errorf("failed to upload metadata for backup %q: %w", chunkName, err)
			}
		}
	} else {
		_, _, err = me.client.drive.UploadFileByPath(ctx, me.Link, archiveName, contentPath, 0)
		if err != nil {
			return fmt.Errorf("failed to upload backup %q: %w", name, err)
		}
	}

	return nil
}

func (me *Folder) ListFilesMetadata(ctx context.Context, instanceID string) ([]string, error) {
	suffix := extendedSuffix(instanceID, "", metadataSuffix)

	files, err := me.client.listFiles(ctx, me.LinkID)
	if err != nil {
		return nil, err
	}

	metadatas := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.HasSuffix(file.Name, suffix) {
			continue
		}

		_, haMetadataJSON, err := me.client.readMetadata(ctx, file.Link)
		if err != nil {
			me.client.logger.WithError(err).Errorf("failed to read metadata file %q", file.Name)
		}
		if haMetadataJSON == "" {
			me.client.logger.Warnf("could not retrieve content of %q, skipping", file.Name)
			continue
		}

		metadatas = append(metadatas, haMetadataJSON)
	}

	return metadatas, nil
}

func (me *Folder) Download(ctx context.Context, instanceID, backupID string) (_ string, ferr error) {
	suffix := extendedSuffix(instanceID, backupID, metadataSuffix)
	metadataFile, err := me.client.findFileInFn(ctx, me.LinkID, func(file *proton_api_bridge.ProtonDirectoryData) bool {
		return strings.HasSuffix(file.Name, suffix)
	})
	if err != nil {
		return "", fmt.Errorf("failed to find metadata file for backup %q: %w", backupID, err)
	}

	metadata, _, err := me.client.readMetadata(ctx, metadataFile.Link)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata file for backup %q: %w", backupID, err)
	}

	file, err := os.CreateTemp("", "*.tar")
	if err != nil {
		return "", fmt.Errorf("failed to create intermediate file: %w", err)
	}
	defer file.Close()
	name := file.Name()
	defer func() {
		if ferr != nil {
			os.Remove(name)
		}
	}()

	if metadata == nil { // legacy, derive the name of the file from the metadata file
		baseName, err := radixFromMetadata(metadataFile.Name)
		if err != nil {
			return "", fmt.Errorf("invalid metadata file name %q: %w", metadataFile.Name, err)
		}

		// radixFromMetadata only removes the extension so the baseName will already have the IDs in this case
		archiveName := makeFileName(baseName, archiveSuffix)

		reader, err := me.client.downloadFile(ctx, me.LinkID, archiveName)
		if err != nil {
			return "", fmt.Errorf("failed to download archive for %q: %w", backupID, err)
		}
		defer reader.Close()

		_, err = io.Copy(file, reader)
		if err != nil {
			return "", fmt.Errorf("failed to copy into intermediate file: %w", err)
		}
	} else {
		chunkNames := make([]string, 0, metadata.Chunks)
		if metadata.Chunks > 1 {
			for i := uint32(0); i < metadata.Chunks; i += 1 {
				chunkNames = append(chunkNames, makeFileName(metadata.BaseName, extendedSuffix(instanceID, backupID, makeChunkSuffix(i))))
			}
		} else {
			chunkNames = append(chunkNames, makeFileName(metadata.BaseName, extendedSuffix(instanceID, backupID, archiveSuffix)))
		}

		for _, chunkName := range chunkNames {
			err := func() error {
				reader, err := me.client.downloadFile(ctx, me.LinkID, chunkName)
				if err != nil {
					return fmt.Errorf("failed to download chunk %q for %q: %w", chunkName, backupID, err)
				}
				defer reader.Close()

				_, err = io.Copy(file, reader)
				if err != nil {
					return fmt.Errorf("failed to copy chunk %q into intermediate file: %w", chunkName, err)
				}
				return nil
			}()
			if err != nil {
				return "", err
			}
		}
	}
	return name, nil
}

func (me *Folder) Delete(ctx context.Context, instanceID, backupID string) error {
	files, err := me.client.listFiles(ctx, me.LinkID)
	if err != nil {
		return err
	}

	archiveBackupSuffix := extendedSuffix(instanceID, backupID, archiveSuffix)
	metadataBackupSuffix := extendedSuffix(instanceID, backupID, metadataSuffix)

	linkIDsToDelete := map[string]string{}
	var metadataLink *proton.Link

	for _, file := range files {
		switch {
		case strings.HasSuffix(file.Name, archiveBackupSuffix): // fallback for legacy backups
			linkIDsToDelete[file.Link.LinkID] = file.Name
		case strings.HasSuffix(file.Name, metadataBackupSuffix):
			linkIDsToDelete[file.Link.LinkID] = file.Name
			metadataLink = file.Link
		}
	}

	if metadataLink == nil {
		me.client.logger.Warnf("metadata file for backup %q not found, cannot check for chunks", backupID)
	} else {
		metadata, _, err := me.client.readMetadata(ctx, metadataLink)
		if err != nil {
			me.client.logger.WithError(err).Errorf("failed to deserialize metadata file %q", metadataLink.Name)
		}

		// only possible for newer metadata
		if metadata != nil {
			filesMap := make(map[string]*proton.Link, len(files))
			for _, file := range files {
				filesMap[file.Name] = file.Link
			}

			archiveFileName := makeFileName(metadata.BaseName, extendedSuffix(instanceID, backupID, archiveBackupSuffix))
			if link, found := filesMap[archiveFileName]; found {
				linkIDsToDelete[link.LinkID] = link.Name
			}

			if metadata.Chunks > 1 {
				for i := uint32(0); i < metadata.Chunks; i += 1 {
					fileName := makeFileName(metadata.BaseName, extendedSuffix(instanceID, backupID, makeChunkSuffix(i)))
					if link, found := filesMap[fileName]; found {
						linkIDsToDelete[link.LinkID] = link.Name
					}
				}
			}
		}
	}

	for linkID, fileName := range linkIDsToDelete {
		me.client.logger.Infof("deleting file %q", fileName)
		err = me.client.drive.MoveFileToTrashByID(ctx, linkID)
		if err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
	}

	return nil
}
