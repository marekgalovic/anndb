package commands

import (
	"io";
	"os";
	"fmt";
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";

	"github.com/satori/go.uuid";
	"github.com/urfave/cli/v2";
	"github.com/olekukonko/tablewriter";
)

func getDatasetManagerClient(c *cli.Context) (pb.DatasetManagerClient, error) {
	conn, err := dialNode(c)
	if err != nil {
		return nil, err
	}

	return pb.NewDatasetManagerClient(conn), nil
}

func ListDatasets(c *cli.Context) error {
	client, err := getDatasetManagerClient(c)
	if err != nil {
		return err
	}

	stream, err := client.List(context.Background(), &pb.EmptyMessage{})
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"id", "dimension", "space", "partition count", "replication factor"})

	for {
		dataset, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		table.Append([]string {
			fmt.Sprintf("%s", uuid.Must(uuid.FromBytes(dataset.GetId()))),
			fmt.Sprintf("%d", dataset.GetDimension()),
			fmt.Sprintf("%s", dataset.GetSpace()),
			fmt.Sprintf("%d", dataset.GetPartitionCount()),
			fmt.Sprintf("%d", dataset.GetReplicationFactor()),
		})
	}

	table.Render()
	return nil
}

func GetDataset(c *cli.Context) error {
	return nil
}