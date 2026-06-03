package apiserver

import (
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ekristen/amihan/pkg/apiserver"
	"github.com/ekristen/amihan/pkg/common"
)

func Execute(c *cli.Context) error {
	return apiserver.RunServer(c.Context, &apiserver.Options{
		Port:             c.Int("port"),
		NomadAddress:     c.String("nomad-address"),
		NomadToken:       c.String("nomad-token"),
		DispatchCooldown: c.Duration("dispatch-cooldown"),
	})
}

func init() {
	flags := []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   4242,
			EnvVars: []string{"PORT"},
		},
		&cli.StringFlag{
			Name:    "nomad-address",
			Usage:   "the http url for the nomad server",
			Value:   "http://localhost:4646",
			EnvVars: []string{"NOMAD_ADDRESS"},
		},
		&cli.StringFlag{
			Name:    "nomad-token",
			Usage:   "the token to use for authenticating to nomad",
			EnvVars: []string{"NOMAD_TOKEN"},
		},
		&cli.DurationFlag{
			Name:    "dispatch-cooldown",
			Usage:   "minimum time before the same firing alert can re-trigger its nomad job",
			Value:   5 * time.Minute,
			EnvVars: []string{"DISPATCH_COOLDOWN"},
		},
	}

	cmd := &cli.Command{
		Name:        "api-server",
		Usage:       "api-server",
		Description: "api-server",
		Before:      common.Before,
		Flags:       append(common.Flags(), flags...),
		Action:      Execute,
	}

	common.RegisterCommand(cmd)
}
