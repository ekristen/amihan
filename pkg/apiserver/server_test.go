package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	nomad "github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/require"
)

func TestNomadDispatcher_Dispatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/job/my-job/dispatch", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"DispatchedJobID":"my-job/dispatch-123","EvalID":"e1"}`))
	}))
	defer ts.Close()

	client, err := nomad.NewClient(&nomad.Config{Address: ts.URL})
	require.NoError(t, err)

	d := &nomadDispatcher{client: client, token: "tok"}
	id, err := d.Dispatch("my-job")
	require.NoError(t, err)
	require.Equal(t, "my-job/dispatch-123", id)
}

func TestNomadDispatcher_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer ts.Close()

	client, err := nomad.NewClient(&nomad.Config{Address: ts.URL})
	require.NoError(t, err)

	d := &nomadDispatcher{client: client, token: "tok"}
	_, err = d.Dispatch("my-job")
	require.Error(t, err)
}

func TestRunServer_StartAndGracefulShutdown(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunServer(ctx, &Options{
			Port:             port,
			NomadAddress:     "http://127.0.0.1:4646",
			DispatchCooldown: time.Minute,
		})
	}()

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(healthURL)
		if err != nil {
			return false
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond, "server did not become ready")

	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("RunServer did not shut down after context cancellation")
	}
}

// freePort returns a TCP port that is free at the moment of the call.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}
