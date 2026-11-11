package entities

import domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"

// NodeRole is a swarm node's role; the text matches Docker's.
type NodeRole string

const (
	NodeRoleWorker  NodeRole = "worker"
	NodeRoleManager NodeRole = "manager"
)

// Validate returns [domainerrors.ErrInvalidNodeRole] unless the role is worker or manager.
func (r NodeRole) Validate() error {
	if r != NodeRoleWorker && r != NodeRoleManager {
		return domainerrors.ErrInvalidNodeRole
	}

	return nil
}

// NodeAvailability tells the scheduler whether a node takes tasks; the text matches Docker's.
type NodeAvailability string

const (
	NodeAvailabilityActive NodeAvailability = "active"
	NodeAvailabilityPause  NodeAvailability = "pause"
	NodeAvailabilityDrain  NodeAvailability = "drain"
)

// Validate returns [domainerrors.ErrInvalidNodeAvailability] unless the availability is active, pause or drain.
func (a NodeAvailability) Validate() error {
	switch a {
	case NodeAvailabilityActive, NodeAvailabilityPause, NodeAvailabilityDrain:
		return nil
	default:
		return domainerrors.ErrInvalidNodeAvailability
	}
}

// Node is a swarm node as the manager sees it.
type Node struct {
	ID           string
	Hostname     string
	Role         NodeRole
	Leader       bool
	State        string // ready, down, disconnected or unknown
	Availability NodeAvailability
	Addr         string
	Engine       string
	OS           string
	Arch         string
	NanoCPUs     int64
	MemoryBytes  int64
}

// JoinInfo is what `docker swarm join` needs.
type JoinInfo struct {
	WorkerToken  string
	ManagerToken string
	// ManagerAddr is the leader's host:port.
	ManagerAddr string
}
