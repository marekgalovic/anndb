package main

import (
	"flag";
	"strings";
	"net";
	"path";
	"time";

	"github.com/marekgalovic/anndb/pkg/storage/raft";
	"github.com/marekgalovic/anndb/pkg/storage/wal";
	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/utils";

	"github.com/satori/go.uuid";
	"google.golang.org/grpc";
	badger "github.com/dgraph-io/badger/v2"
	log "github.com/sirupsen/logrus";
)

func main() {
	var port string
	var joinNodesRaw string
	var dataDir string
	flag.StringVar(&port, "port", "6000", "Node port")
	flag.StringVar(&joinNodesRaw, "join", "", "Comma separated list of existing cluster nodes")
	flag.StringVar(&dataDir, "data-dir", "/tmp", "Data directory")
	flag.Parse()

	db, err := badger.Open(badger.DefaultOptions(path.Join(dataDir, "anndb")))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	raftNodeId, err := getRaftNodeId(db)
	if err != nil {
		log.Fatal(err)
	}

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
	zeroGroup, err := raft.NewRaftGroup(uuid.Nil, getZeroNodeIds(raftTransport.NodeId(), joinNodesRaw), wal.NewBadgerWAL(db, raftNodeId, uuid.Nil), raftTransport)
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

func getRaftNodeId(db *badger.DB) (uint64, error) {
	nodeId, err := wal.GetBadgerRaftId(db)
	if err != nil && err != badger.ErrKeyNotFound {
		return 0, err
	} else if err == nil {
		return nodeId, nil
	}

	nodeId = uint64(time.Now().UTC().UnixNano())
	if err := wal.SetBadgerRaftId(db, nodeId); err != nil {
		return 0, err
	}

	return nodeId, nil
}

func getZeroNodeIds(nodeId uint64, join string) []uint64 {
	if len(join) == 0 {
		return []uint64{nodeId}
	}

	return nil
}