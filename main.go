package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"github.com/maxlaverse/ndots-admission-controller/pkg"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func main() {
	debugMode := false
	var keyPair pkg.KeyPair

	app := &cli.App{
		Name:  "ndots-admission-controller",
		Usage: "AdmissionController that sets ndots=1 on Pods",
		Before: func(c *cli.Context) error {
			fs := flag.NewFlagSet("", flag.PanicOnError)
			klog.InitFlags(fs)
			return fs.Set("v", strconv.Itoa(c.Int("loglevel")))
		},
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
			}, &cli.IntFlag{
				Name:    "loglevel",
				Aliases: []string{"v"},
				Usage:   "Log Level",
				Value:   0,
			},
		},
		Action: func(c *cli.Context) error {
			defer klog.Flush()

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
