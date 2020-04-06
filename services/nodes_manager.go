package services

import (
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/storage/raft";
)

type nodesManagerServer struct {
	nodesManager *raft.NodesManager
}

func NewNodesManagerServer(nodesManager *raft.NodesManager) *nodesManagerServer {
	return &nodesManagerServer {
		nodesManager: nodesManager,
	}
}

func (this *nodesManagerServer) ListNodes(req *pb.EmptyMessage, stream pb.NodesManager_ListNodesServer) error {
	for id, address := range this.nodesManager.ListNodes() {
		if err := stream.Send(&pb.Node{Id: id, Address: address}); err != nil {
			return err
		}	
	}
	return nil
}

func (this *nodesManagerServer) AddNode(req *pb.Node, stream pb.NodesManager_AddNodeServer) error {
	nodes, err := this.nodesManager.AddNode(req.GetId(), req.GetAddress())
	if err != nil {
		return err
	}

	for nodeId, address := range nodes {
		if err := stream.Send(&pb.Node{Id: nodeId, Address: address}); err != nil {
			return err
		}
	}

	return nil
}

func (this *nodesManagerServer) RemoveNode(ctx context.Context, req *pb.Node) (*pb.EmptyMessage, error) {
	if err := this.nodesManager.RemoveNode(req.GetId()); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}