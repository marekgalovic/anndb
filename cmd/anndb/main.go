package main

import (
	"flag";
	"strings";
	"net";

	"github.com/marekgalovic/anndb/pkg/storage/raft";
	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/utils";

	"github.com/satori/go.uuid";
	etcdRaft "github.com/coreos/etcd/raft";

	"google.golang.org/grpc";
	log "github.com/sirupsen/logrus";
)

func main() {
	var raftNodeId uint64
	var port string
	var joinNodesRaw string
	var dataDir string
	flag.Uint64Var(&raftNodeId, "id", 1, "Raft node id")
	flag.StringVar(&port, "port", "6000", "Node port")
	flag.StringVar(&joinNodesRaw, "join", "", "Comma separated list of existing cluster nodes")
	flag.StringVar(&dataDir, "data-dir", "/tmp", "Data directory")
	flag.Parse()

	raftTransport := raft.NewTransport(raftNodeId, net.JoinHostPort("", port))

	// Start server
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRaftTransportServer(grpcServer, raftTransport)
	go grpcServer.Serve(listener)

	// Start raft zero group
	zeroGroup, err := raft.NewRaftGroup(uuid.Nil, getZeroNodeIds(raftTransport.NodeId(), joinNodesRaw), etcdRaft.NewMemoryStorage(), raftTransport)
	if err != nil {
		log.Fatal(err)
	}
	defer zeroGroup.Stop()

	if len(joinNodesRaw) > 0 {
		if err := zeroGroup.Join(strings.Split(joinNodesRaw, ",")); err != nil {
			log.Fatal(err)
		}
	}

	// Shutdown
	<- utils.InterruptSignal()
	grpcServer.GracefulStop()
	listener.Close()
	log.Info("Shutdown")
}

func getZeroNodeIds(nodeId uint64, join string) []uint64 {
	if len(join) == 0 {
		return []uint64{nodeId}
	}

	return nil
}