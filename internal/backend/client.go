package backend

import (
	"fmt"

	rolev1 "mxd-battle/internal/gen/mxd/role/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn

	Role rolev1.RoleServiceClient
}

func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect backend grpc: %w", err)
	}

	return &Client{
		conn: conn,
		Role: rolev1.NewRoleServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
