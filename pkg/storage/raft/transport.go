package raft

import (
	"context";
	"sync";
	"errors";

	pb "github.com/marekgalovic/anndb/pkg/protobuf";

	"github.com/satori/go.uuid";
	"google.golang.org/grpc";
	"github.com/golang/protobuf/proto";
	// "go.etcd.io/etcd/raft";
	"go.etcd.io/etcd/raft/raftpb";
	log "github.com/sirupsen/logrus";
)

var (
	GroupAlreadyExistsError error = errors.New("Group already exists")
	GroupNotFoundError error = errors.New("Group not found")
	NodeAddressNotFoundError error = errors.New("Node address not found")
)

type raftTransport struct {
	nodeAddresses map[uint64]string
	nodeAddressesMu sync.RWMutex
	nodeConns map[uint64]*grpc.ClientConn
	nodeConnsMu sync.RWMutex
	nodeClients map[uint64]pb.RaftTransportClient
	nodeClientsMu sync.RWMutex

	groups map[uuid.UUID]*raftGroup
	groupsMu sync.RWMutex
}

func NewTransport(join []string) (*raftTransport, error) {
	t := &raftTransport {
		nodeAddresses: make(map[uint64]string),
		nodeAddressesMu: sync.RWMutex{},
		nodeConns: make(map[uint64]*grpc.ClientConn),
		nodeConnsMu: sync.RWMutex{},
		nodeClients: make(map[uint64]pb.RaftTransportClient),
		nodeClientsMu: sync.RWMutex{},

		groups: make(map[uuid.UUID]*raftGroup),
		groupsMu: sync.RWMutex{},
	}

	return t, nil
}

func (this *raftTransport) AddGroup(group *raftGroup) error {
	this.groupsMu.Lock()
	defer this.groupsMu.Unlock()

	if _, exists := this.groups[group.id]; exists {
		return GroupAlreadyExistsError
	}

	this.groups[group.id] = group
	return nil
}

func (this *raftTransport) RemoveGroup(id uuid.UUID) error {
	this.groupsMu.Lock()
	defer this.groupsMu.Unlock()

	if _, exists := this.groups[id]; !exists {
		return GroupNotFoundError
	}

	delete(this.groups, id)
	return nil
}

func (this *raftTransport) GetGroup(id uuid.UUID) (*raftGroup, error) {
	this.groupsMu.RLock()
	defer this.groupsMu.RUnlock()

	group, exists := this.groups[id]
	if !exists {
		return nil, GroupNotFoundError
	}
	return group, nil
}

func (this *raftTransport) Receive(ctx context.Context, req *pb.RaftMessage) (*pb.EmptyMessage, error) {
	return &pb.EmptyMessage{}, nil
}

func (this *raftTransport) Send(ctx context.Context, group *raftGroup, messages []raftpb.Message) {
	for nodeId, nodeMessages := range this.groupMessagesByRecipient(&messages) {
		payload, err := this.buildRaftMessage(group.id, nodeMessages)
		if err != nil {
			log.Error(err)
			continue
		}

		client, err := this.getNodeRaftTransportClient(nodeId)
		if err != nil {
			log.Warn(err)
			group.reportUnreachable(nodeId)
			continue
		}

		if _, err := client.Receive(ctx, payload); err != nil {
			log.Warn(err)
			group.reportUnreachable(nodeId)
		}
	}
}

func (this *raftTransport) groupMessagesByRecipient(messages *[]raftpb.Message) map[uint64][]*raftpb.Message {
	grouped := make(map[uint64][]*raftpb.Message)
	for _, message := range *messages {
		if _, exists := grouped[message.To]; !exists {
			grouped[message.To] = make([]*raftpb.Message, 0)
		}
		grouped[message.To] = append(grouped[message.To], &message)
	}
	return grouped
}

func (this *raftTransport) buildRaftMessage(groupId uuid.UUID, messages []*raftpb.Message) (*pb.RaftMessage, error) {
	var err error
	messageBytes := make([][]byte, len(messages))
	for i, message := range messages {
		messageBytes[i], err = proto.Marshal(message)
		if err != nil {
			return nil, err
		}
	}

	return &pb.RaftMessage {
		GroupId: groupId.Bytes(),
		Messages: messageBytes,
	}, nil
}

func (this *raftTransport) getNodeRaftTransportClient(nodeId uint64) (pb.RaftTransportClient, error) {
	this.nodeClientsMu.RLock()
	if client, exists := this.nodeClients[nodeId]; exists {
		this.nodeClientsMu.RUnlock()
		return client, nil
	}
	this.nodeClientsMu.RUnlock()

	conn, err := this.getNodeConn(nodeId)
	if err != nil {
		return nil, err
	}

	this.nodeClientsMu.Lock()
	defer this.nodeClientsMu.RUnlock()

	this.nodeClients[nodeId] = pb.NewRaftTransportClient(conn)
	return this.nodeClients[nodeId], nil
}

func (this *raftTransport) getNodeConn(nodeId uint64) (*grpc.ClientConn, error) {
	this.nodeConnsMu.RLock()
	if conn, exists := this.nodeConns[nodeId]; exists {
		this.nodeConnsMu.RUnlock()
		return conn, nil
	}
	this.nodeConnsMu.RUnlock()

	this.nodeAddressesMu.RLock()
	address, exists := this.nodeAddresses[nodeId]
	if !exists {
		this.nodeAddressesMu.RUnlock()
		return nil, NodeAddressNotFoundError
	}
	this.nodeAddressesMu.RUnlock()

	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	this.nodeConnsMu.Lock()
	defer this.nodeConnsMu.Unlock()

	if existingConn, exists := this.nodeConns[nodeId]; exists {
		conn.Close()
		return existingConn, nil
	}

	this.nodeConns[nodeId] = conn
	return conn, nil
}