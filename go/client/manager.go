package client

import (
	"github.com/ProtonMail/go-proton-api"
	"github.com/sirupsen/logrus"
)

func NewManager(logger *logrus.Logger) *proton.Manager {
	return proton.New(
		proton.WithLogger(logger),
		proton.WithDebug(logger.Level == logrus.DebugLevel),
		proton.WithAppVersion("Other"),
	)
}
