package app

import (
	"context"

	"github.com/rotisserie/eris"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
)

var _ ports.Nodes = (*Nodes)(nil)

type Nodes struct {
	swarm ports.Swarm
}

func NewNodes(swarm ports.Swarm) *Nodes {
	return &Nodes{swarm: swarm}
}

// ListNodes implements [ports.Nodes].
func (n *Nodes) ListNodes(ctx context.Context, actor *entities.User) ([]entities.Node, error) {
	if err := authorizeRole(actor, false); err != nil {
		return nil, err
	}

	nodes, err := n.swarm.ListNodes(ctx)

	return nodes, eris.Wrap(err, "list nodes")
}

// JoinInfo implements [ports.Nodes]. Admin only although it reads: a token lets any host join the swarm.
func (n *Nodes) JoinInfo(ctx context.Context, actor *entities.User) (*entities.JoinInfo, error) {
	if err := authorizeRole(actor, true); err != nil {
		return nil, err
	}

	info, err := n.swarm.JoinInfo(ctx)

	return info, eris.Wrap(err, "join info")
}

// SetNodeRole implements [ports.Nodes].
func (n *Nodes) SetNodeRole(ctx context.Context, actor *entities.User, id string, role entities.NodeRole) error {
	if err := authorizeRole(actor, true); err != nil {
		return err
	}

	if err := role.Validate(); err != nil {
		return err
	}

	return eris.Wrap(n.swarm.SetNodeRole(ctx, id, role), "set node role")
}

// SetNodeAvailability implements [ports.Nodes].
func (n *Nodes) SetNodeAvailability(
	ctx context.Context,
	actor *entities.User,
	id string,
	availability entities.NodeAvailability,
) error {
	if err := authorizeRole(actor, true); err != nil {
		return err
	}

	if err := availability.Validate(); err != nil {
		return err
	}

	return eris.Wrap(n.swarm.SetNodeAvailability(ctx, id, availability), "set node availability")
}

// RemoveNode implements [ports.Nodes].
func (n *Nodes) RemoveNode(ctx context.Context, actor *entities.User, id string, force bool) error {
	if err := authorizeRole(actor, true); err != nil {
		return err
	}

	return eris.Wrap(n.swarm.RemoveNode(ctx, id, force), "remove node")
}
