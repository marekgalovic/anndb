package services

import (
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/storage/raft";

	"github.com/shirou/gopsutil/host";
	"github.com/shirou/gopsutil/load";
	"github.com/shirou/gopsutil/mem";
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

func (this *nodesManagerServer) LoadInfo(ctx context.Context, req *pb.EmptyMessage) (*pb.NodeLoadInfo, error) {
	hInfoStat, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	vMemStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	loadAvgStat, err := load.AvgWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.NodeLoadInfo {
		Uptime: hInfoStat.Uptime,
		CpuLoad1: loadAvgStat.Load1,
		CpuLoad5: loadAvgStat.Load5,
		CpuLoad15: loadAvgStat.Load15,
		MemTotal: vMemStat.Total,
		MemAvailable: vMemStat.Available,
		MemUsed: vMemStat.Used,
		MemFree: vMemStat.Free,
		MemUsedPercent: vMemStat.UsedPercent,
	}, nil
}