package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/maxlaverse/ndots-admission-controller/pkg"
	"github.com/maxlaverse/ndots-admission-controller/pkg/log"
	"github.com/op/go-logging"
	"github.com/urfave/cli/v2"
)

func main() {
	debugMode := false
	var keyPair pkg.KeyPair

	app := &cli.App{
		Name:  "ndots-admission-controller",
		Usage: "AdmissionController that sets ndots=1 on Pods",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Value:       false,
				Usage:       "Output additional debug lines",
				Destination: &debugMode,
			}, &cli.StringFlag{
				Name:        "tls-cert-file",
				Value:       "/etc/certs/tls.crt",
				Usage:       "TLS certificate",
				Destination: &keyPair.TLSCertFilepath,
			}, &cli.StringFlag{
				Name:        "tls-key-file",
				Value:       "/etc/certs/tls.key",
				Usage:       "TLS key",
				Destination: &keyPair.TLSKeyFilepath,
			},
		},
		Before: configureLogging(&debugMode),
		Action: func(c *cli.Context) error {
			server := pkg.NewWebhookServer(keyPair)

			ctx, stop := signal.NotifyContext(c.Context, os.Interrupt)
			defer stop()

			return server.Run(ctx)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func configureLogging(debugMode *bool) cli.BeforeFunc {
	return func(_ *cli.Context) error {
		logFormat := logging.MustStringFormatter(`%{time:15:04:05.000} ▶ %{level:.5s} %{message}`)

		backend := logging.NewLogBackend(os.Stderr, "", 0)
		formatter := logging.NewBackendFormatter(backend, logFormat)
		leveledBackend := logging.AddModuleLevel(formatter)
		if *debugMode {
			leveledBackend.SetLevel(logging.DEBUG, "")
		} else {
			leveledBackend.SetLevel(logging.INFO, "")
		}

		logger := &logging.Logger{}
		logger.SetBackend(leveledBackend)
		log.SetLogger(logger)
		return nil
	}
}
