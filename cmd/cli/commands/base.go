package commands

import (
	"net";

	"github.com/urfave/cli/v2";
	"google.golang.org/grpc";
)

func dialNode(c *cli.Context) (*grpc.ClientConn, error) {
	host := c.String("host")
	port := c.String("port")
	if port == "" {
		port = "6001"
	}

	return grpc.Dial(net.JoinHostPort(host, port), grpc.WithInsecure())
}