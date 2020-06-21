package commands

import (
	"io";
	"os";
	"fmt";
	"strings";
	"context";
	"errors";

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

	stream, err := client.List(context.Background(), &pb.ListDatasetsRequest{WithSize: true})
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"id", "size", "dimension", "space", "partition count", "replication factor"})

	for {
		dataset, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		table.Append([]string {
			fmt.Sprintf("%s", uuid.Must(uuid.FromBytes(dataset.GetId()))),
			fmt.Sprintf("%d", dataset.GetSize()),
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
	if c.NArg() == 0 {
		return errors.New("No dataset id provided")
	}
	id, err := uuid.FromString(c.Args().Get(0))
	if err != nil {
		return err
	}

	client, err := getDatasetManagerClient(c)
	if err != nil {
		return err
	}

	dataset, err := client.Get(context.Background(), &pb.GetDatasetRequest{DatasetId: id.Bytes(), WithSize: false})
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"partition id", "node ids"})

	for _, partition := range dataset.GetPartitions() {
		nodes := make([]string, len(partition.GetNodeIds()))
		for i, id := range partition.GetNodeIds() {
			nodes[i] = fmt.Sprintf("%16x", id)
		}
		table.Append([]string {
			fmt.Sprintf("%s", uuid.Must(uuid.FromBytes(partition.GetId()))),
			strings.Join(nodes, ","),
		})
	}

	table.Render()
	return nil
}

func CreateDataset(c *cli.Context) error {
	dim := c.Uint("dim")
	rawSpace := c.String("space")
	partitionCount := c.Uint("partition-count")
	replicationFactor := c.Uint("replication-factor")

	space, err := spaceStringToSpaceProto(rawSpace)
	if err != nil {
		return err
	}

	client, err := getDatasetManagerClient(c)
	if err != nil {
		return err
	}

	dataset, err := client.Create(context.Background(), &pb.Dataset {
		Dimension: uint32(dim),
		Space: space,
		PartitionCount: uint32(partitionCount),
		ReplicationFactor: uint32(replicationFactor),

	})
	if err != nil {
		return err
	}

	fmt.Printf("Created dataset %s\n", uuid.Must(uuid.FromBytes(dataset.GetId())))
	return nil
}

func DeleteDataset(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("Missing dataset id")
	}

	id, err := uuid.FromString(c.Args().Get(0))
	if err != nil {
		return err
	}

	client, err := getDatasetManagerClient(c)
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &pb.UUIDRequest {
		Id: id.Bytes(),
	})
	return err
}

func spaceStringToSpaceProto(raw string) (pb.Space, error) {
	switch raw {
	case "euclidean":
		return pb.Space_Euclidean, nil
	case "manhattan":
		return pb.Space_Manhattan, nil
	case "cosine":
		return pb.Space_Cosine, nil
	default:
		return pb.Space_Euclidean, errors.New("Invalid space")
	}
}