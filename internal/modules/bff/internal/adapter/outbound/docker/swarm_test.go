package docker_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/docker"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

// fakeEngine serves the Engine API with handler; paths drop the /v1.xx version prefix.
func fakeEngine(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, path string)) *docker.Swarm {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, path, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		handler(w, r, "/"+path)
	}))
	t.Cleanup(srv.Close)

	return newSwarm(t, "tcp://"+srv.Listener.Addr().String())
}

func newSwarm(t *testing.T, host string) *docker.Swarm {
	t.Helper()

	cli, err := client.New(client.WithHost(host), client.WithAPIVersion("1.47"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	return docker.NewSwarm(cli)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestListNodes(t *testing.T) {
	t.Parallel()

	s := fakeEngine(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		worker := swarm.Node{ID: "w1", Spec: swarm.NodeSpec{Role: swarm.NodeRoleWorker}}
		worker.Description.Hostname = "a-worker"
		mgr := swarm.Node{
			ID:            "m1",
			Spec:          swarm.NodeSpec{Role: swarm.NodeRoleManager, Availability: swarm.NodeAvailabilityDrain},
			ManagerStatus: &swarm.ManagerStatus{Leader: true, Addr: "10.0.0.1:2377"},
			Status:        swarm.NodeStatus{State: swarm.NodeStateReady, Addr: "10.0.0.1"},
		}
		mgr.Description.Hostname = "z-manager"
		mgr.Description.Resources.MemoryBytes = 8 << 30
		mgr.Description.Platform.OS = "linux"
		writeJSON(w, http.StatusOK, []swarm.Node{worker, mgr})
	})

	nodes, err := s.ListNodes(t.Context())
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	assert.Equal(t, entities.Node{
		ID: "m1", Hostname: "z-manager", Role: entities.NodeRoleManager, Leader: true, State: "ready",
		Availability: entities.NodeAvailabilityDrain, Addr: "10.0.0.1", OS: "linux", MemoryBytes: 8 << 30,
	}, nodes[0], "managers first")
	assert.Equal(t, "w1", nodes[1].ID)
}

func TestSetNodeAvailability_KeepsSpecAndVersion(t *testing.T) {
	t.Parallel()

	var got swarm.NodeSpec

	var version string

	s := fakeEngine(t, func(w http.ResponseWriter, r *http.Request, path string) {
		switch path {
		case "/nodes/n1":
			n := swarm.Node{ID: "n1", Spec: swarm.NodeSpec{Role: swarm.NodeRoleManager}}
			n.Version.Index = 5
			n.Spec.Labels = map[string]string{"zone": "a"}
			writeJSON(w, http.StatusOK, n)
		case "/nodes/n1/update":
			version = r.URL.Query().Get("version")
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	})

	require.NoError(t, s.SetNodeAvailability(t.Context(), "n1", entities.NodeAvailabilityDrain))
	assert.Equal(t, "5", version)
	assert.Equal(t, swarm.NodeAvailabilityDrain, got.Availability)
	assert.Equal(t, swarm.NodeRoleManager, got.Role)
	assert.Equal(t, map[string]string{"zone": "a"}, got.Labels)
}

func TestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		msg    string
		want   error
		reason string
	}{
		{name: "not found", status: http.StatusNotFound, msg: "node n1 not found", want: domainerrors.ErrNodeNotFound},
		{name: "rejected", status: http.StatusBadRequest,
			msg:    "rpc error: code = FailedPrecondition desc = node n1 is not down and can't be removed",
			reason: "node n1 is not down and can't be removed"},
		{name: "conflict", status: http.StatusConflict, msg: "update out of sequence", reason: "update out of sequence"},
		{name: "not a manager", status: http.StatusServiceUnavailable, msg: "This node is not a swarm manager.",
			want: domainerrors.ErrSwarmUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fakeEngine(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
				writeJSON(w, tt.status, map[string]string{"message": tt.msg})
			})

			err := s.RemoveNode(t.Context(), "n1", false)

			if tt.reason != "" {
				var rejected domainerrors.NodeRejectedError
				require.ErrorAs(t, err, &rejected)
				assert.Equal(t, tt.reason, rejected.Reason)

				return
			}

			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestConnectionFailed(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.Listener.Addr().String()
	srv.Close()

	_, err := newSwarm(t, "tcp://"+addr).ListNodes(t.Context())
	assert.ErrorIs(t, err, domainerrors.ErrSwarmUnavailable)
}
