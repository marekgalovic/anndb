package anndb

type Config struct {
	RaftNodeId uint64
	DataDir string
	Port string
	JoinNodes []string
	TlsCertFile string
	TlsKeyFile string
}

func NewConfig() *Config {
	return &Config {
		Port: "6000",
		DataDir: "/anndb_data",
	}
}