package main

import (
	"os";

	"github.com/marekgalovic/anndb/cmd/cli/commands";

	"github.com/urfave/cli/v2";
	log "github.com/sirupsen/logrus";
)

func main() {
	app := &cli.App {
		Name: "anndb-cli",
		Flags: []cli.Flag {
			&cli.StringFlag{Name: "host", Aliases: []string{"H"}},
			&cli.StringFlag{Name: "port", Aliases: []string{"p"}},
		},
		Commands: []*cli.Command {
			{
				Name: "nodes",
				Usage: "Manage cluster nodes",
				Subcommands: []*cli.Command {
					{
						Name: "list",
						Usage: "List cluster nodes",
						Action: commands.ListNodes,
					},
					{
						Name: "add",
						Usage: "Add new node to the cluster",
						Action: commands.AddNode,
					},
					{
						Name: "remove",
						Usage: "Remove cluster node",
						Action: commands.RemoveNode,
					},
				},
			},
			{
				Name: "datasets",
				Subcommands: []*cli.Command {
					{
						Name: "list",
						Usage: "List datasets",
						Action: commands.ListDatasets,
					},
					{
						Name: "get",
						Usage: "Get dataset",
						Action: commands.GetDataset,
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}