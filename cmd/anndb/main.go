package main

import (
	"flag";
	"strings";
	"net";

	"github.com/marekgalovic/anndb/pkg/storage/raft";
	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/utils";

	"google.golang.org/grpc";
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

	joinNodes := strings.Split(joinNodesRaw, ",")
	raftTransport, err := raft.NewTransport(joinNodes)
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	listener, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRaftTransportServer(grpcServer, raftTransport)
	go grpcServer.Serve(listener)

	// Shutdown
	<- utils.InterruptSignal()
	grpcServer.GracefulStop()
	listener.Close()
	log.Info("Shutdown")
}