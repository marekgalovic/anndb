package commands

import (
	"io";
	"os";
	"fmt";
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";

	"github.com/urfave/cli/v2";
	"github.com/olekukonko/tablewriter";
)

func getNodesManagerClient(c *cli.Context) (pb.NodesManagerClient, error) {
	conn, err := dialNode(c)
	if err != nil {
		return nil, err
	}

	return pb.NewNodesManagerClient(conn), nil
}

func ListNodes(c *cli.Context) error {
	client, err := getNodesManagerClient(c)
	if err != nil {
		return err
	}

	stream, err := client.ListNodes(context.Background(), &pb.EmptyMessage{})
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"id", "address"})

	for {
		node, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		table.Append([]string{
			fmt.Sprintf("%d", node.GetId()),
			node.GetAddress(),
		})
	}

	table.Render()
	return nil
}

func AddNode(c *cli.Context) error {
	return nil
}

func RemoveNode(c *cli.Context) error {
	return nil
}