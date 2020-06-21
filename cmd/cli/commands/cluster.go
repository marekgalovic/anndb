package commands

import (
	"io";
	"os";
	"fmt";
	"context";
	"net";
	"strconv";
	"errors";
	"time";

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

func getNodesManagerClientForAddr(addr string) (pb.NodesManagerClient, error) {
	conn, err := dialNodeByAddr(addr)
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
	table.SetHeader([]string{"id", "address", "uptime", "load", "used memory"})

	for {
		node, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		nodeClient, err := getNodesManagerClientForAddr(node.GetAddress())
		if err != nil {
			return err
		}
		loadInfo, err := nodeClient.LoadInfo(context.Background(), &pb.EmptyMessage{})
		if err != nil {
			return err
		}

		table.Append([]string{
			fmt.Sprintf("%16x", node.GetId()),
			node.GetAddress(),
			fmt.Sprintf("%s", time.Duration(loadInfo.GetUptime()) * time.Second),
			fmt.Sprintf("%.2f%%", loadInfo.GetCpuLoad5()),
			fmt.Sprintf("%.2f%%", loadInfo.GetMemUsedPercent()),
		})
	}

	table.Render()
	return nil
}

func AddNode(c *cli.Context) error {
	idRaw := c.String("node-id")
	address := c.String("node-address")
	port := c.String("node-port")

	if idRaw == "" {
		return errors.New("Must provide node id")
	}
	if address == "" {
		return errors.New("Must provide node address")
	}
	if port == "" {
		return errors.New("Must provide node port")
	}
	id, err := strconv.ParseUint(idRaw, 16, 64)
	if err != nil {
		return err
	}

	client, err := getNodesManagerClient(c)
	if err != nil {
		return err
	}

	_, err = client.AddNode(context.Background(), &pb.Node{Id: id, Address: net.JoinHostPort(address, port)})
	return err
}

func RemoveNode(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("No node id provided")
	}

	nodeId, err := strconv.ParseUint(c.Args().Get(0), 16, 64)
	if err != nil {
		return err
	}

	client, err := getNodesManagerClient(c)
	if err != nil {
		return err
	}

	_, err = client.RemoveNode(context.Background(), &pb.Node{Id: nodeId})
	return err
}
