package raft

import (
	"io";
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/cluster";

	log "github.com/sirupsen/logrus";
)

type NodesManager struct {
	clusterConn *cluster.Conn
	zeroGroup *RaftGroup
}

func NewNodesManager(clusterConn *cluster.Conn, zeroGroup *RaftGroup) *NodesManager {
	return &NodesManager {
		clusterConn: clusterConn,
		zeroGroup: zeroGroup,
	}
}

func (this *NodesManager) Join(ctx context.Context, addresses []string) error {
	for i, addr := range addresses {
		err := this.tryJoin(ctx, addr)
		if err != nil {
			if i == len(addresses) - 1 {
				log.Errorf("Join attempt failed. %v", err)
				continue
			} else {
				return err
			}
		}
		break
	}
	return nil
}

func (this *NodesManager) AddNode(id uint64, address string) (map[uint64]string, error) {
	if err := this.zeroGroup.ProposeJoin(id, address); err != nil {
		return nil, err
	}

	nodes := this.clusterConn.Nodes()
	nodes[id] = address
	return nodes, nil
}

func (this *NodesManager) RemoveNode(id uint64) error {
	return this.zeroGroup.ProposeLeave(id)
}

func (this *NodesManager) tryJoin(ctx context.Context, address string) error {
	conn, err := this.clusterConn.DialAddress(address)
	if err != nil {
		return err
	}
	defer conn.Close()

	nodesStream, err := pb.NewNodesManagerClient(conn).AddNode(ctx, &pb.Node {
		Id: this.clusterConn.Id(),
		Address: this.clusterConn.Address(),
	})
	if err != nil {
		return err
	}

	for {
		node, err := nodesStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		this.clusterConn.AddNode(node.GetId(), node.GetAddress())
	}
	return nil
}