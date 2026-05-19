package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LouisBrunner/gopy-ha-proton-drive/go/client"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

type result struct {
	// Updated on login or if the auth changes, always check it!
	Creds *client.Credentials `json:"creds"`
	// Provided on `download`
	DownloadedPath *string `json:"downloaded_path"`
	// Provided on `list-shares`
	Shares []client.Share `json:"shares"`
	// Provided on `list-metadata`
	Metadata []string `json:"metadata"`
}

func prepareClient(ctx context.Context, logger *logrus.Logger, cmd *cli.Command, onAuthChange client.OnAuthChange, partialOpts *client.Options) (*client.Client, *client.Folder, error) {
	if partialOpts == nil {
		partialOpts = &client.Options{}
	}
	clt, err := client.New(ctx, client.Options{
		Logger: logger,
		Credentials: client.Credentials{
			UID:           cmd.String("uid"),
			AccessToken:   cmd.String("access-token"),
			RefreshToken:  cmd.String("refresh-token"),
			SaltedKeyPass: cmd.String("salted-key-pass"),
		},
		OnAuthChange:         onAuthChange,
		MaxUploadTries:       partialOpts.MaxUploadTries,
		UploadChunkSizeBytes: partialOpts.UploadChunkSizeBytes,
		ShareID:              cmd.String("share-id"),
	})
	if err != nil {
		return nil, nil, err
	}
	folder, err := clt.MakeRootFolder(ctx, cmd.String("root-folder"))
	if err != nil {
		return nil, nil, err
	}
	return clt, folder, nil
}

func work(ctx context.Context, logger *logrus.Logger, args []string) (*result, error) {
	var err error
	var res result
	credUpdater := func(newCreds client.Credentials) {
		logger.Infof("Credentials automatically renewed by Proton")
		res.Creds = &newCreds
	}

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "log-level",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			logLevelString := cmd.String("log-level")
			if logLevelString != "" {
				logLevel, err := logrus.ParseLevel(logLevelString)
				if err != nil {
					return nil, err
				}
				logger.SetLevel(logLevel)
			}
			return ctx, nil
		},
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
						Name: "mailbox-password",
					},
					&cli.StringFlag{
						Name: "mfa",
					},
					&cli.StringFlag{
						Name: "captcha-token",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					res.Creds, err = client.Login(ctx, client.NewManager(logger), client.LoginOptions{
						Username:        cmd.String("email"),
						Password:        cmd.String("password"),
						MailboxPassword: cmd.String("mailbox-password"),
						TwoFA:           cmd.String("mfa"),
						CaptchaToken:    cmd.String("captcha-token"),
					})
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
							_, _, err := prepareClient(ctx, logger, cmd, credUpdater, nil)
							return err
						},
					},
					{
						Name: "download",
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
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater, nil)
							if err != nil {
								return err
							}
							path, err := folder.Download(ctx, cmd.String("instance-id"), cmd.String("backup-id"))
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
								Name:     "instance-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "backup-id",
								Required: true,
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater, nil)
							if err != nil {
								return err
							}
							return folder.Delete(ctx, cmd.String("instance-id"), cmd.String("backup-id"))
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
							&cli.Uint64Flag{
								Name:  "chunk-size",
								Usage: "Upload chunk size in bytes",
							},
							&cli.IntFlag{
								Name:  "max-tries",
								Usage: "Maximum number of tries for each chunk upload",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater, &client.Options{
								MaxUploadTries:       cmd.Int("max-tries"),
								UploadChunkSizeBytes: cmd.Uint64("chunk-size"),
							})
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
							_, folder, err := prepareClient(ctx, logger, cmd, credUpdater, nil)
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
							client, _, err := prepareClient(ctx, logger, cmd, credUpdater, nil)
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
		code := "unknown"
		switch {
		case errors.Is(err, client.ErrMFARequired):
			code = "mfa"
		case errors.Is(err, client.ErrMailboxPassRequired):
			code = "two-pass"
		}
		fmt.Printf(`{"error":{"message":%q,"code":%q}}`, err.Error(), code)
	}
}
