package raft

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/marekgalovic/anndb/cluster"
	pb "github.com/marekgalovic/anndb/protobuf"

	etcdRaft "github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/golang/protobuf/proto"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var (
	GroupAlreadyExistsError error = errors.New("Group already exists")
	GroupNotFoundError      error = errors.New("Group not found")
)

type RaftTransport struct {
	nodeId      uint64
	address     string
	clusterConn *cluster.Conn

	nodeClients   map[uint64]pb.RaftTransportClient
	nodeClientsMu sync.RWMutex
	groups        map[uuid.UUID]*RaftGroup
	groupsMu      sync.RWMutex
}

func NewTransport(nodeId uint64, address string, clusterConn *cluster.Conn) *RaftTransport {
	clusterConn.AddNode(nodeId, address)

	return &RaftTransport{
		nodeId:      nodeId,
		address:     address,
		clusterConn: clusterConn,

		nodeClients:   make(map[uint64]pb.RaftTransportClient),
		nodeClientsMu: sync.RWMutex{},
		groups:        make(map[uuid.UUID]*RaftGroup),
		groupsMu:      sync.RWMutex{},
	}
}

func (this *RaftTransport) NodeId() uint64 {
	return this.nodeId
}

func (this *RaftTransport) Address() string {
	return this.address
}

func (this *RaftTransport) Receive(ctx context.Context, req *pb.RaftMessage) (*pb.EmptyMessage, error) {
	groupId, err := uuid.FromBytes(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	group, err := this.getGroup(groupId)
	if err != nil {
		return nil, err
	}

	var message raftpb.Message
	if err := proto.Unmarshal(req.GetMessage(), &message); err != nil {
		return nil, err
	}

	if err := group.receive(message); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}

func (this *RaftTransport) Send(ctx context.Context, group *RaftGroup, messages []raftpb.Message) {
	for _, m := range messages {
		mBytes, err := m.Marshal()
		if err != nil {
			log.Error(err)
			continue
		}

		client, err := this.getNodeRaftTransportClient(m.To)
		if err != nil {
			group.reportUnreachable(m.To)
			if m.Type == raftpb.MsgSnap {
				group.reportSnapshot(m.To, etcdRaft.SnapshotFailure)
			}
			continue
		}

		payload := &pb.RaftMessage{
			GroupId: group.id.Bytes(),
			Message: mBytes,
		}

		sendCtx, _ := context.WithTimeout(ctx, 500*time.Millisecond)
		if _, err := client.Receive(sendCtx, payload); err != nil {
			group.reportUnreachable(m.To)
			if m.Type == raftpb.MsgSnap {
				group.reportSnapshot(m.To, etcdRaft.SnapshotFailure)
			}
			continue
		}

		if m.Type == raftpb.MsgSnap {
			group.reportSnapshot(m.To, etcdRaft.SnapshotFinish)
		}
	}
}

func (this *RaftTransport) addNodeAddress(nodeId uint64, address string) {
	this.clusterConn.AddNode(nodeId, address)
}

func (this *RaftTransport) removeNodeAddress(nodeId uint64) {
	this.clusterConn.RemoveNode(nodeId)
}

func (this *RaftTransport) addGroup(group *RaftGroup) error {
	this.groupsMu.Lock()
	defer this.groupsMu.Unlock()

	if _, exists := this.groups[group.id]; exists {
		return GroupAlreadyExistsError
	}

	this.groups[group.id] = group
	return nil
}

func (this *RaftTransport) removeGroup(id uuid.UUID) error {
	this.groupsMu.Lock()
	defer this.groupsMu.Unlock()

	if _, exists := this.groups[id]; !exists {
		return GroupNotFoundError
	}

	delete(this.groups, id)
	return nil
}

func (this *RaftTransport) getGroup(id uuid.UUID) (*RaftGroup, error) {
	this.groupsMu.RLock()
	defer this.groupsMu.RUnlock()

	group, exists := this.groups[id]
	if !exists {
		return nil, GroupNotFoundError
	}
	return group, nil
}

func (this *RaftTransport) getNodeRaftTransportClient(nodeId uint64) (pb.RaftTransportClient, error) {
	this.nodeClientsMu.RLock()
	if client, exists := this.nodeClients[nodeId]; exists {
		this.nodeClientsMu.RUnlock()
		return client, nil
	}
	this.nodeClientsMu.RUnlock()

	conn, err := this.clusterConn.Dial(nodeId)
	if err != nil {
		return nil, err
	}

	this.nodeClientsMu.Lock()
	defer this.nodeClientsMu.Unlock()

	this.nodeClients[nodeId] = pb.NewRaftTransportClient(conn)
	return this.nodeClients[nodeId], nil
}
