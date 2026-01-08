package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	proton "github.com/LouisBrunner/gopy-ha-proton-drive/go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

type result struct {
	// Updated on login or if the auth changes, always check it!
	Creds *proton.Credentials `json:"creds"`
	// Provided on `find`
	LinkID *string `json:"link_id"`
	// Provided on `download`
	DownloadedPath *string `json:"downloaded_path"`
	// Provided on `list-shares`
	Shares []proton.Share `json:"shares"`
	// Provided on `list-metadata`
	Metadata []string `json:"metadata"`
}

func prepareClient(ctx context.Context, logger *logrus.Logger, cmd *cli.Command, onAuthChange proton.OnAuthChange) (*proton.Client, *proton.Folder, error) {
	client, err := proton.NewClient(ctx, logger, proton.Credentials{
		UID:           cmd.String("uid"),
		AccessToken:   cmd.String("access-token"),
		RefreshToken:  cmd.String("refresh-token"),
		SaltedKeyPass: cmd.String("salted-key-pass"),
	}, onAuthChange)
	if err != nil {
		return nil, nil, err
	}
	shareID := cmd.String("share-id")
	if shareID != "" {
		err = client.SelectShare(ctx, shareID)
		if err != nil {
			return nil, nil, err
		}
	}
	rootFolder := cmd.String("root-folder")
	folder, err := client.MakeRootFolder(ctx, rootFolder)
	if err != nil {
		return nil, nil, err
	}
	return client, folder, nil
}

func work(ctx context.Context, logger *logrus.Logger, args []string) (*result, error) {
	var err error
	var res result
	credUpdater := func(newCreds proton.Credentials) {
		logger.Infof("Credentials automatically renewed by Proton")
		res.Creds = &newCreds
	}

	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name: "login",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "email",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "password",
						Required: true,
					},
					&cli.StringFlag{
						Name: "mfa",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					res.Creds, err = proton.Login(ctx, cmd.String("email"), cmd.String("password"), cmd.String("mfa"))
					return err
				},
			},
			{
				Name: "with-creds",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "uid",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "access-token",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "refresh-token",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "salted-key-pass",
						Required: true,
					},
					&cli.StringFlag{
						Name: "share-id",
					},
					&cli.StringFlag{
						Name: "root-folder",
					},
				},
				Commands: []*cli.Command{
					{
						Name: "check",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, _, err := prepareClient(ctx, logger, cmd, credUpdater)
							return err
						},
					},
					{
						Name: "download",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "link-id",
								Required: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							client, _, err := prepareClient(ctx, logger, cmd, credUpdater)
							if err != nil {
								return err
							}
							path, err := client.DownloadFile(ctx, cmd.String("link-id"))
							if err != nil {
								return err
							}
							res.DownloadedPath = &path
							return nil
						},
					},
					{
						Name: "delete",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "link-id",
								Required: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							client, _, err := prepareClient(ctx, logger, cmd, credUpdater)
							if err != nil {
								return err
							}
							return client.DeleteFile(ctx, cmd.String("link-id"))
						},
					},
					{
						Name: "find",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "instance-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "backup-id",
								Required: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater)
							if err != nil {
								return err
							}
							linkID, err := folder.FindBackup(ctx, cmd.String("instance-id"), cmd.String("backup-id"))
							if err != nil {
								return err
							}
							res.LinkID = &linkID
							return nil
						},
					},
					{
						Name: "upload",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "instance-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "backup-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "metadata-json",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "content-path",
								Required: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater)
							if err != nil {
								return err
							}
							return folder.Upload(ctx, cmd.String("instance-id"), cmd.String("backup-id"), cmd.String("name"), cmd.String("metadata-json"), cmd.String("content-path"))
						},
					},
					{
						Name: "list-metadata",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "instance-id",
								Required: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater)
							if err != nil {
								return err
							}
							res.Metadata, err = folder.ListFilesMetadata(ctx, cmd.String("instance-id"))
							return err
						},
					},
					{
						Name: "list-shares",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							client, _, err := prepareClient(ctx, logger, cmd, credUpdater)
							if err != nil {
								return err
							}
							res.Shares, err = client.ListShares(ctx)
							return err
						},
					},
				},
			},
		},
	}

	err = cmd.Run(ctx, args)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func wrapper(ctx context.Context, logger *logrus.Logger, args []string) error {
	res, err := work(ctx, logger, args)
	if err != nil {
		return err
	}
	resJSON, err := json.Marshal(res)
	if err != nil {
		return err
	}
	fmt.Println(string(resJSON))
	return nil
}

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stderr)
	logger.SetReportCaller(true)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := wrapper(ctx, logger, os.Args)
	if err != nil {
		logger.WithError(err).Error("failed")
		fmt.Printf(`{"error": %q}`, err.Error())
	}
}
