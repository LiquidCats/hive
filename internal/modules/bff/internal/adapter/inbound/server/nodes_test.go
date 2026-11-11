package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/inbound/server"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

func TestNodes_Access(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		as         string
		method     string
		path       string
		body       any
		expect     func(h *hive) // swarm calls the request makes; none means the mock fails on any
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{name: "user lists", as: "user", method: http.MethodGet, path: "/nodes",
			expect: func(h *hive) {
				h.swarm.EXPECT().ListNodes(mock.Anything).Return([]entities.Node{{ID: "n1"}}, nil)
			}, wantStatus: http.StatusOK},
		{name: "user cannot read join tokens", as: "user", method: http.MethodGet, path: "/nodes/join",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "user cannot drain", as: "user", method: http.MethodPut, path: "/nodes/n1/availability",
			body: map[string]any{"availability": "drain"}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "user cannot promote", as: "user", method: http.MethodPut, path: "/nodes/n1/role",
			body: map[string]any{"role": "manager"}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "user cannot remove", as: "user", method: http.MethodDelete, path: "/nodes/n1",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},

		{name: "admin reads join tokens", as: "admin", method: http.MethodGet, path: "/nodes/join",
			expect: func(h *hive) {
				h.swarm.EXPECT().JoinInfo(mock.Anything).Return(&entities.JoinInfo{ManagerAddr: "10.0.0.1:2377"}, nil)
			}, wantStatus: http.StatusOK},
		{name: "admin drains", as: "admin", method: http.MethodPut, path: "/nodes/n1/availability",
			body: map[string]any{"availability": "drain"},
			expect: func(h *hive) {
				h.swarm.EXPECT().SetNodeAvailability(mock.Anything, "n1", entities.NodeAvailabilityDrain).Return(nil)
			}, wantStatus: http.StatusNoContent},
		{name: "admin promotes", as: "admin", method: http.MethodPut, path: "/nodes/n1/role",
			body: map[string]any{"role": "manager"},
			expect: func(h *hive) {
				h.swarm.EXPECT().SetNodeRole(mock.Anything, "n1", entities.NodeRoleManager).Return(nil)
			}, wantStatus: http.StatusNoContent},
		{name: "admin force removes", as: "admin", method: http.MethodDelete, path: "/nodes/n1?force=true",
			expect: func(h *hive) {
				h.swarm.EXPECT().RemoveNode(mock.Anything, "n1", true).Return(nil)
			}, wantStatus: http.StatusNoContent},

		{name: "invalid role", as: "admin", method: http.MethodPut, path: "/nodes/n1/role",
			body: map[string]any{"role": "owner"}, wantStatus: http.StatusBadRequest, wantCode: "invalid_node_role"},
		{name: "invalid availability", as: "admin", method: http.MethodPut, path: "/nodes/n1/availability",
			body:       map[string]any{"availability": "off"},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_node_availability"},
		{name: "malformed body", as: "admin", method: http.MethodPut, path: "/nodes/n1/role", body: "{",
			wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "unknown node", as: "admin", method: http.MethodDelete, path: "/nodes/n1",
			expect: func(h *hive) {
				h.swarm.EXPECT().RemoveNode(mock.Anything, "n1", false).Return(domainerrors.ErrNodeNotFound)
			}, wantStatus: http.StatusNotFound, wantCode: "node_not_found"},
		{name: "docker rejects with its reason", as: "admin", method: http.MethodDelete, path: "/nodes/n1",
			expect: func(h *hive) {
				h.swarm.EXPECT().RemoveNode(mock.Anything, "n1", false).
					Return(eris.Wrap(domainerrors.NodeRejectedError{Reason: "node n1 is not down"}, "docker"))
			}, wantStatus: http.StatusConflict, wantCode: "node_rejected", wantMsg: "node n1 is not down"},
		{name: "swarm unavailable", as: "user", method: http.MethodGet, path: "/nodes",
			expect: func(h *hive) {
				h.swarm.EXPECT().ListNodes(mock.Anything).Return(nil, domainerrors.ErrSwarmUnavailable)
			}, wantStatus: http.StatusServiceUnavailable, wantCode: "swarm_unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newHive(t)
			if tt.expect != nil {
				tt.expect(h)
			}

			rec := h.do(t, tt.as, tt.method, tt.path, tt.body)
			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())

			if tt.wantCode != "" {
				var body server.Error
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tt.wantCode, body.Code)

				if tt.wantMsg != "" {
					assert.Equal(t, tt.wantMsg, body.Message)
				}
			}
		})
	}
}
