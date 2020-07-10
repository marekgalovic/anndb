package anndb

import (
	"net";
	"path";
	"context";
	"time";
	"errors";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/cluster";
	"github.com/marekgalovic/anndb/storage/raft";
	"github.com/marekgalovic/anndb/storage/wal";
	"github.com/marekgalovic/anndb/storage";
	"github.com/marekgalovic/anndb/services";

	"github.com/satori/go.uuid";
	"google.golang.org/grpc";
	"google.golang.org/grpc/credentials";
	badger "github.com/dgraph-io/badger/v2"
	log "github.com/sirupsen/logrus";
)

type Server struct {
	config *Config

	db *badger.DB
	clusterConn *cluster.Conn
	allocator *storage.Allocator
	zeroGroup *raft.RaftGroup
	datasetManager *storage.DatasetManager
	nodesManager *raft.NodesManager


	grpcServer *grpc.Server
	listener net.Listener

}

func NewServer(config *Config) *Server {
	s := &Server {
		config: config,
	}

	return s
}

func (this *Server) Stop() error {
	this.grpcServer.GracefulStop()
	this.listener.Close()
	this.datasetManager.Close()
	this.zeroGroup.Stop()
	this.allocator.Stop()
	this.clusterConn.Close()
	this.db.Close()
	return nil
}

func (this *Server) Run() error {
	if err := this.setup(); err != nil {
		return err
	}

	go this.grpcServer.Serve(this.listener)
	return nil
}

func (this *Server) JoinCluster() error {
	log.Info(this.config.JoinNodes)
	if len(this.config.JoinNodes) == 0 {
		return nil
	}

	return this.nodesManager.Join(context.Background(), this.config.JoinNodes)
}

func (this *Server) setup() error {
	var err error

	this.db, err = badger.Open(badger.LSMOnlyOptions(path.Join(this.config.DataDir, "anndb")).WithLogger(log.New()))
	if err != nil {
		return err
	}

	this.config.RaftNodeId, err = this.getRaftNodeId(this.config.RaftNodeId)
	if err != nil {
		return err
	}

	this.clusterConn, err = cluster.NewConn(this.config.RaftNodeId, net.JoinHostPort("", this.config.Port), this.config.TlsCertFile)
	if err != nil {
		return err
	}

	this.allocator = storage.NewAllocator(this.clusterConn) 

	raftTransport := raft.NewTransport(this.config.RaftNodeId, net.JoinHostPort("", this.config.Port), this.clusterConn)

	// Start raft zero group
	this.zeroGroup, err = raft.NewRaftGroup(uuid.Nil, this.getZeroNodeIds(raftTransport.NodeId()), wal.NewBadgerWAL(this.db, uuid.Nil), raftTransport)
	if err != nil {
		return err
	}
 
	// Create shared raft group on top of the zero group
	sharedGroup, err := raft.NewSharedGroup(this.zeroGroup)
	if err != nil {
		return err
	}

	this.nodesManager = raft.NewNodesManager(this.clusterConn, this.zeroGroup)

	this.datasetManager, err = storage.NewDatasetManager(sharedGroup.Get("datasets"), this.db, raftTransport, this.clusterConn, this.allocator)
	if err != nil {
		return err
	}

	// Start server
	this.listener, err = net.Listen("tcp", net.JoinHostPort("", this.config.Port))
	if err != nil {
		return err
	}

	grpcServerOptions := make([]grpc.ServerOption, 0)
	if (this.config.TlsCertFile != "") && (this.config.TlsKeyFile != "") {
		creds, err := credentials.NewServerTLSFromFile(this.config.TlsCertFile, this.config.TlsKeyFile)
		if err != nil {
			return err
		}
		grpcServerOptions = append(grpcServerOptions, grpc.Creds(creds))
		log.Info("Using SSL")
	}

	this.grpcServer = grpc.NewServer(grpcServerOptions...)
	pb.RegisterRaftTransportServer(this.grpcServer, raftTransport)
	pb.RegisterNodesManagerServer(this.grpcServer, services.NewNodesManagerServer(this.nodesManager))
	pb.RegisterDatasetManagerServer(this.grpcServer, services.NewDatasetManagerServer(this.datasetManager))
	pb.RegisterDataManagerServer(this.grpcServer, services.NewDataManagerServer(this.datasetManager))
	pb.RegisterSearchServer(this.grpcServer, services.NewSearchServer(this.datasetManager))

	return nil
}

func (this *Server) getRaftNodeId(existingId uint64) (uint64, error) {
	nodeId, err := wal.GetBadgerRaftId(this.db)
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
	if err := wal.SetBadgerRaftId(this.db, nodeId); err != nil {
		return 0, err
	}

	return nodeId, nil
}

func (this *Server) getZeroNodeIds(nodeId uint64) []uint64 {
	if len(this.config.JoinNodes) == 0 {
		return []uint64{nodeId}
	}

	return nil
}