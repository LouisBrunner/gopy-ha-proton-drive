package proton

import (
	"context"
	"fmt"
	"io"
	"os"
)

func (me *Client) DownloadFile(ctx context.Context, linkID string) (_ string, ferr error) {
	link, err := me.drive.GetLink(ctx, linkID)
	if err != nil {
		return "", fmt.Errorf("failed to find file: %w", err)
	}

	reader, _, _, err := me.drive.DownloadFile(ctx, link, 0)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer reader.Close()

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

	_, err = io.Copy(file, reader)
	if err != nil {
		return "", fmt.Errorf("failed to copy into intermediate file: %w", err)
	}
	return name, nil
}

func (me *Client) DeleteFile(ctx context.Context, linkID string) error {
	link, err := me.drive.GetLink(ctx, linkID)
	if err != nil {
		return fmt.Errorf("failed to find file: %w", err)
	}
	name, err := me.getName(ctx, link)
	if err != nil {
		return fmt.Errorf("failed to get file name: %w", err)
	}

	metadataName, err := metadataFromArchive(name)
	if err != nil {
		return fmt.Errorf("could not find metadata file: %w", err)
	}

	metadataLink, err := me.findFileIn(ctx, link.ParentLinkID, metadataName)
	if err != nil {
		return fmt.Errorf("failed to find metadata file: %w", err)
	}

	for _, lid := range []string{linkID, metadataLink.LinkID} {
		err = me.drive.MoveFileToTrashByID(ctx, lid)
		if err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
	}

	return nil
}
