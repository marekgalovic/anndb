package main

import (
	"os";
	"flag";
	"strings";

	"github.com/marekgalovic/anndb";
	"github.com/marekgalovic/anndb/utils";

	log "github.com/sirupsen/logrus";
)

func main() {
	var joinNodesRaw string

	config := anndb.NewConfig()
	flag.Uint64Var(&config.RaftNodeId, "node-id", 0, "Raft node ID")
	flag.StringVar(&config.Port, "port", utils.GetenvDefault("ANNDB_PORT", "6000"), "Node port")
	flag.StringVar(&joinNodesRaw, "join", utils.GetenvDefault("ANNDB_JOIN", ""), "Comma separated list of existing cluster nodes")
	flag.StringVar(&config.DataDir, "data-dir", utils.GetenvDefault("ANNDB_DATA_DIR", "/tmp"), "Data directory")
	flag.StringVar(&config.TlsCertFile, "cert", utils.GetenvDefault("ANNDB_CERT", ""), "TLS Certificate file")
	flag.StringVar(&config.TlsKeyFile, "key", utils.GetenvDefault("ANNDB_KEY", ""), "TLS Key file")
	flag.Parse()

	if joinNodesRaw == "false" {
		config.DoNotJoinCluster = true
	} else {
		config.JoinNodes = splitCommaSeparatedValues(joinNodesRaw)
	}

	if _, err := os.Stat(config.DataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(config.DataDir, os.ModePerm); err != nil {
			log.Fatal("Failed to create data directory.")
		}
	}

	server := anndb.NewServer(config)
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
	defer server.Stop()


	if !config.DoNotJoinCluster {
		if err := server.JoinCluster(); err != nil {
			log.Fatal(err)
		}
	}

	// Shutdown
	<- utils.InterruptSignal()
	log.Info("Shutdown")
}

func splitCommaSeparatedValues(raw string) []string {
	result := make([]string, 0)
	for _, s := range strings.Split(raw, ",") {
		if len(s) > 0 {
			result = append(result, strings.Trim(s, " "))
		}
	}
	return result
}