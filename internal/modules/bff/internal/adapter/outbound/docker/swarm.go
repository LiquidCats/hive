package docker

import (
	"cmp"
	"context"
	"slices"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/rotisserie/eris"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

var _ ports.Swarm = (*Swarm)(nil)

// Swarm manages nodes through the Docker Engine API of a swarm manager.
type Swarm struct {
	client *client.Client
}

func NewSwarm(cli *client.Client) *Swarm {
	return &Swarm{client: cli}
}

// ListNodes implements [ports.Swarm].
func (s *Swarm) ListNodes(ctx context.Context) ([]entities.Node, error) {
	res, err := s.client.NodeList(ctx, client.NodeListOptions{})
	if err != nil {
		return nil, mapError(err)
	}

	nodes := make([]entities.Node, len(res.Items))
	for i, n := range res.Items {
		nodes[i] = toNode(n)
	}

	slices.SortFunc(nodes, func(a, b entities.Node) int {
		// "manager" < "worker"
		return cmp.Or(cmp.Compare(a.Role, b.Role), cmp.Compare(a.Hostname, b.Hostname))
	})

	return nodes, nil
}

// JoinInfo implements [ports.Swarm].
func (s *Swarm) JoinInfo(ctx context.Context) (*entities.JoinInfo, error) {
	res, err := s.client.SwarmInspect(ctx, client.SwarmInspectOptions{})
	if err != nil {
		return nil, mapError(err)
	}

	nodes, err := s.client.NodeList(ctx, client.NodeListOptions{
		Filters: make(client.Filters).Add("role", string(swarm.NodeRoleManager)),
	})
	if err != nil {
		return nil, mapError(err)
	}

	info := &entities.JoinInfo{
		WorkerToken:  res.Swarm.JoinTokens.Worker,
		ManagerToken: res.Swarm.JoinTokens.Manager,
	}

	for _, n := range nodes.Items {
		if n.ManagerStatus != nil && n.ManagerStatus.Leader {
			info.ManagerAddr = n.ManagerStatus.Addr
		}
	}

	return info, nil
}

// SetNodeRole implements [ports.Swarm].
func (s *Swarm) SetNodeRole(ctx context.Context, id string, role entities.NodeRole) error {
	return s.update(ctx, id, func(spec *swarm.NodeSpec) { spec.Role = swarm.NodeRole(role) })
}

// SetNodeAvailability implements [ports.Swarm].
func (s *Swarm) SetNodeAvailability(ctx context.Context, id string, availability entities.NodeAvailability) error {
	return s.update(ctx, id, func(spec *swarm.NodeSpec) { spec.Availability = swarm.NodeAvailability(availability) })
}

// RemoveNode implements [ports.Swarm].
func (s *Swarm) RemoveNode(ctx context.Context, id string, force bool) error {
	_, err := s.client.NodeRemove(ctx, id, client.NodeRemoveOptions{Force: force})

	return mapError(err)
}

// update is a read-modify-write of the node spec.
// ponytail: no retry when another writer bumps the version in between; Docker's "update out of sequence"
// reaches the admin as a rejection, retry once here if that happens in practice.
func (s *Swarm) update(ctx context.Context, id string, change func(*swarm.NodeSpec)) error {
	res, err := s.client.NodeInspect(ctx, id, client.NodeInspectOptions{})
	if err != nil {
		return mapError(err)
	}

	spec := res.Node.Spec
	change(&spec)

	_, err = s.client.NodeUpdate(ctx, id, client.NodeUpdateOptions{Version: res.Node.Version, Spec: spec})

	return mapError(err)
}

func toNode(n swarm.Node) entities.Node {
	return entities.Node{
		ID:           n.ID,
		Hostname:     n.Description.Hostname,
		Role:         entities.NodeRole(n.Spec.Role),
		Leader:       n.ManagerStatus != nil && n.ManagerStatus.Leader,
		State:        string(n.Status.State),
		Availability: entities.NodeAvailability(n.Spec.Availability),
		Addr:         n.Status.Addr,
		Engine:       n.Description.Engine.EngineVersion,
		OS:           n.Description.Platform.OS,
		Arch:         n.Description.Platform.Architecture,
		NanoCPUs:     n.Description.Resources.NanoCPUs,
		MemoryBytes:  n.Description.Resources.MemoryBytes,
	}
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case cerrdefs.IsNotFound(err):
		return eris.Wrap(domainerrors.ErrNodeNotFound, err.Error())
	case cerrdefs.IsInvalidArgument(err), cerrdefs.IsConflict(err), cerrdefs.IsFailedPrecondition(err):
		return eris.Wrap(domainerrors.NodeRejectedError{Reason: reason(err)}, "docker")
	case cerrdefs.IsUnavailable(err), client.IsErrConnectionFailed(err):
		return eris.Wrap(domainerrors.ErrSwarmUnavailable, err.Error())
	default:
		return eris.Wrap(err, "docker")
	}
}

// reason drops the "Error response from daemon: rpc error: code = ... desc =" prefix Docker puts on swarm errors.
func reason(err error) string {
	msg := err.Error()
	if _, desc, ok := strings.Cut(msg, "desc = "); ok {
		return desc
	}

	return strings.TrimPrefix(msg, "Error response from daemon: ")
}
