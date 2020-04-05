package raft

import (
	"context";
	"sync";
	"errors";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/cluster";

	"github.com/satori/go.uuid";
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
	"github.com/golang/protobuf/proto";
	log "github.com/sirupsen/logrus";
)

var (
	GroupAlreadyExistsError error = errors.New("Group already exists")
	GroupNotFoundError error = errors.New("Group not found")
)

type RaftTransport struct {
	nodeId uint64
	address string
	clusterConn *cluster.Conn

	nodeClients map[uint64]pb.RaftTransportClient
	nodeClientsMu sync.RWMutex
	groups map[uuid.UUID]*RaftGroup
	groupsMu sync.RWMutex
}

func NewTransport(nodeId uint64, address string, clusterConn *cluster.Conn) *RaftTransport {
	clusterConn.AddNode(nodeId, address)

	return &RaftTransport {
		nodeId: nodeId,
		address: address,
		clusterConn: clusterConn,

		nodeClients: make(map[uint64]pb.RaftTransportClient),
		nodeClientsMu: sync.RWMutex{},
		groups: make(map[uuid.UUID]*RaftGroup),
		groupsMu: sync.RWMutex{},
	}
}

func (this *RaftTransport) NodeId() uint64 {
	return this.nodeId
}

func (this *RaftTransport) Address() string {
	return this.address
}

func (this *RaftTransport) ProposeJoin(req *pb.RaftJoinMessage, stream pb.RaftTransport_ProposeJoinServer) error {
	groupId, err := uuid.FromBytes(req.GetGroupId())
	if err != nil {
		return err
	}
	group, err := this.getGroup(groupId)
	if err != nil {
		return err
	}

	if err := group.proposeJoin(req.GetNodeId(), req.GetAddress()); err != nil {
		return err
	}

	if err := stream.Send(&pb.RaftNode{Id: this.NodeId(), Address: this.Address()}); err != nil {
		return err
	}

	for nodeId, address := range this.clusterConn.Nodes() {
		if err := stream.Send(&pb.RaftNode{Id: nodeId, Address: address}); err != nil {
			return err
		}
	}

	return nil
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

	messages := make([]raftpb.Message, len(req.GetMessages()))
	for i, msgBytes := range req.GetMessages() {
		if err := proto.Unmarshal(msgBytes, &messages[i]); err != nil {
			return nil, err
		}
	}

	if err := group.receive(messages); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}

func (this *RaftTransport) Send(ctx context.Context, group *RaftGroup, messages []raftpb.Message) {
	for nodeId, nodeMessages := range this.groupMessagesByRecipient(&messages) {
		payload, containsSnapshot, err := this.buildRaftMessage(group.id, nodeMessages)
		if err != nil {
			log.Error(err)
			continue
		}

		client, err := this.getNodeRaftTransportClient(nodeId)
		if err != nil {
			log.Warn(err)
			group.reportUnreachable(nodeId)
			if containsSnapshot {
				group.reportSnapshot(nodeId, etcdRaft.SnapshotFailure)
			}
			continue
		}

		if _, err := client.Receive(ctx, payload); err != nil {
			group.reportUnreachable(nodeId)
			if containsSnapshot {
				group.reportSnapshot(nodeId, etcdRaft.SnapshotFailure)
			}
			continue
		}
		if containsSnapshot {
			group.reportSnapshot(nodeId, etcdRaft.SnapshotFinish)
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

func (this *RaftTransport) groupMessagesByRecipient(messages *[]raftpb.Message) map[uint64][]*raftpb.Message {
	grouped := make(map[uint64][]*raftpb.Message)
	for _, message := range *messages {
		if _, exists := grouped[message.To]; !exists {
			grouped[message.To] = make([]*raftpb.Message, 0)
		}
		grouped[message.To] = append(grouped[message.To], &message)
	}
	return grouped
}

func (this *RaftTransport) buildRaftMessage(groupId uuid.UUID, messages []*raftpb.Message) (*pb.RaftMessage, bool, error) {
	var err error
	var containsSnapshot bool = false
	messageBytes := make([][]byte, len(messages))
	for i, message := range messages {
		containsSnapshot = containsSnapshot || (message.Type == raftpb.MsgSnap)
		messageBytes[i], err = proto.Marshal(message)
		if err != nil {
			return nil, containsSnapshot, err
		}
	}

	return &pb.RaftMessage {
		GroupId: groupId.Bytes(),
		Messages: messageBytes,
	}, containsSnapshot, nil
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