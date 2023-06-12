package apiserver

import (
	"github.com/ekristen/amihan/pkg/apiserver"
	"github.com/ekristen/amihan/pkg/common"
	"github.com/urfave/cli/v2"
)

func Execute(c *cli.Context) error {
	return apiserver.RunServer(c.Context, &apiserver.Options{
		Port:         c.Int("port"),
		NomadAddress: c.String("nomad-address"),
		NomadToken:   c.String("nomad-token"),
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
