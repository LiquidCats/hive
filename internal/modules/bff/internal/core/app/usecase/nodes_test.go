package app_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"

	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	"github.com/LiquidCats/hive/internal/modules/bff/tests/mocks"
)

type NodesSuite struct {
	suite.Suite

	swarm *mocks.MockSwarm
	nodes *app.Nodes
	admin *entities.User
	user  *entities.User
}

func TestNodesSuite(t *testing.T) {
	suite.Run(t, new(NodesSuite))
}

func (s *NodesSuite) SetupTest() {
	s.swarm = mocks.NewMockSwarm(s.T())
	s.nodes = app.NewNodes(s.swarm)
	s.admin = &entities.User{ID: uuid.NewV7(), Role: values.RoleAdmin}
	s.user = &entities.User{ID: uuid.NewV7(), Role: values.RoleUser}
}

// Refused before any swarm call; the mock fails on any call.
func (s *NodesSuite) TestForbidden() {
	adminOnly := map[string]func(ctx context.Context, actor *entities.User) error{
		"JoinInfo": func(ctx context.Context, actor *entities.User) error {
			_, err := s.nodes.JoinInfo(ctx, actor)

			return err
		},
		"SetNodeRole": func(ctx context.Context, actor *entities.User) error {
			return s.nodes.SetNodeRole(ctx, actor, "n1", entities.NodeRoleManager)
		},
		"SetNodeAvailability": func(ctx context.Context, actor *entities.User) error {
			return s.nodes.SetNodeAvailability(ctx, actor, "n1", entities.NodeAvailabilityDrain)
		},
		"RemoveNode": func(ctx context.Context, actor *entities.User) error {
			return s.nodes.RemoveNode(ctx, actor, "n1", true)
		},
	}

	for name, call := range adminOnly {
		for _, actor := range []*entities.User{nil, s.user} {
			s.Require().ErrorIs(call(s.T().Context(), actor), domainerrors.ErrForbidden, name)
		}
	}

	_, err := s.nodes.ListNodes(s.T().Context(), nil)
	s.Require().ErrorIs(err, domainerrors.ErrForbidden)
}

func (s *NodesSuite) TestInvalidInput() {
	ctx := s.T().Context()

	s.Require().ErrorIs(s.nodes.SetNodeRole(ctx, s.admin, "n1", "owner"), domainerrors.ErrInvalidNodeRole)
	s.Require().ErrorIs(s.nodes.SetNodeAvailability(ctx, s.admin, "n1", "off"), domainerrors.ErrInvalidNodeAvailability)
}

func (s *NodesSuite) TestPassesThrough() {
	ctx := s.T().Context()
	nodes := []entities.Node{{ID: "n1"}}

	s.swarm.EXPECT().ListNodes(ctx).Return(nodes, nil)
	s.swarm.EXPECT().SetNodeRole(ctx, "n1", entities.NodeRoleWorker).Return(domainerrors.ErrNodeNotFound)
	s.swarm.EXPECT().SetNodeAvailability(ctx, "n1", entities.NodeAvailabilityPause).Return(nil)
	s.swarm.EXPECT().RemoveNode(ctx, "n1", false).Return(nil)
	s.swarm.EXPECT().JoinInfo(ctx).Return(&entities.JoinInfo{WorkerToken: "w"}, nil)

	got, err := s.nodes.ListNodes(ctx, s.user)
	s.Require().NoError(err)
	s.Equal(nodes, got)

	s.Require().ErrorIs(s.nodes.SetNodeRole(ctx, s.admin, "n1", entities.NodeRoleWorker), domainerrors.ErrNodeNotFound)
	s.NoError(s.nodes.SetNodeAvailability(ctx, s.admin, "n1", entities.NodeAvailabilityPause))
	s.NoError(s.nodes.RemoveNode(ctx, s.admin, "n1", false))

	info, err := s.nodes.JoinInfo(ctx, s.admin)
	s.Require().NoError(err)
	s.Equal("w", info.WorkerToken)
}
