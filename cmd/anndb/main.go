package main

import (
	"flag";
	"strings";
	"net";
	"path";
	"time";
	"context";
	"errors";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/cluster";
	"github.com/marekgalovic/anndb/storage/raft";
	"github.com/marekgalovic/anndb/storage/wal";
	"github.com/marekgalovic/anndb/storage";
	"github.com/marekgalovic/anndb/services";
	"github.com/marekgalovic/anndb/utils";

	"github.com/satori/go.uuid";
	"google.golang.org/grpc";
	"google.golang.org/grpc/credentials";
	badger "github.com/dgraph-io/badger/v2"
	log "github.com/sirupsen/logrus";
)

func main() {
	var raftNodeId uint64
	var port string
	var joinNodesRaw string
	var dataDir string
	var tlsCertFile string
	var tlsKeyFile string
	flag.Uint64Var(&raftNodeId, "node-id", 0, "Raft node ID")
	flag.StringVar(&port, "port", utils.GetenvDefault("ANNDB_PORT", "6000"), "Node port")
	flag.StringVar(&joinNodesRaw, "join", utils.GetenvDefault("ANNDB_JOIN", ""), "Comma separated list of existing cluster nodes")
	flag.StringVar(&dataDir, "data-dir", utils.GetenvDefault("ANNDB_DATA_DIR", "/tmp"), "Data directory")
	flag.StringVar(&tlsCertFile, "cert", utils.GetenvDefault("ANNDB_CERT", ""), "TLS Certificate file")
	flag.StringVar(&tlsKeyFile, "key", utils.GetenvDefault("ANNDB_KEY", ""), "TLS Key file")
	flag.Parse()

	db, err := badger.Open(badger.LSMOnlyOptions(path.Join(dataDir, "anndb")).WithLogger(log.New()))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	raftNodeId, err = getRaftNodeId(db, raftNodeId)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Raft node id: %16x", raftNodeId)

	clusterConn, err := cluster.NewConn(raftNodeId, net.JoinHostPort("", port), tlsCertFile)
	if err != nil {
		log.Fatal(err)
	}
	defer clusterConn.Close()

	allocator := storage.NewAllocator(clusterConn)
	defer allocator.Stop()

	raftTransport := raft.NewTransport(raftNodeId, net.JoinHostPort("", port), clusterConn)

	// Start raft zero group
	zeroGroup, err := raft.NewRaftGroup(uuid.Nil, getZeroNodeIds(raftTransport.NodeId(), joinNodesRaw), wal.NewBadgerWAL(db, uuid.Nil), raftTransport)
	if err != nil {
		log.Fatal(err)
	}
	defer zeroGroup.Stop()
	// Create shared raft group on top of the zero group
	sharedGroup, err := raft.NewSharedGroup(zeroGroup)
	if err != nil {
		log.Fatal(err)
	}

	nodesManager := raft.NewNodesManager(clusterConn, zeroGroup)

	datasetManager, err := storage.NewDatasetManager(sharedGroup.Get("datasets"), db, raftTransport, clusterConn, allocator)
	if err != nil {
		log.Fatal(err)
	}
	defer datasetManager.Close()

	// Start server
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	grpcServerOptions := make([]grpc.ServerOption, 0)
	if tlsCertFile != "" && tlsKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(tlsCertFile, tlsKeyFile)
		if err != nil {
			log.Fatal(err)
		}
		grpcServerOptions = append(grpcServerOptions, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(grpcServerOptions...)
	pb.RegisterRaftTransportServer(grpcServer, raftTransport)
	pb.RegisterNodesManagerServer(grpcServer, services.NewNodesManagerServer(nodesManager))
	pb.RegisterDatasetManagerServer(grpcServer, services.NewDatasetManagerServer(datasetManager))
	pb.RegisterDataManagerServer(grpcServer, services.NewDataManagerServer(datasetManager))
	pb.RegisterSearchServer(grpcServer, services.NewSearchServer(datasetManager))
	go grpcServer.Serve(listener)

	// Join cluster
	if (len(joinNodesRaw) > 0) && (joinNodesRaw != "false") {
		if err := nodesManager.Join(context.Background(), strings.Split(joinNodesRaw, ",")); err != nil {
			log.Fatal(err)
		}
	}

	// Shutdown
	<- utils.InterruptSignal()
	grpcServer.GracefulStop()
	log.Info("Shutdown")
}

func getRaftNodeId(db *badger.DB, existingId uint64) (uint64, error) {
	nodeId, err := wal.GetBadgerRaftId(db)
	if err != nil && err != badger.ErrKeyNotFound {
		return 0, err
	} else if err == nil {
		if existingId > 0 && existingId != nodeId {
			return 0, errors.New("Starting node with custom id which does not match the one stored in WAL")
		}
		return nodeId, nil
	}

	if existingId > 0 {
		nodeId = existingId
	} else {
		nodeId = uint64(time.Now().UTC().UnixNano())
	}
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