package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	nomad "github.com/hashicorp/nomad/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type Options struct {
	Port             int
	NomadAddress     string
	NomadToken       string
	DispatchCooldown time.Duration

	Log *logrus.Entry
}

// Dispatcher dispatches a parameterized Nomad job and returns the dispatched job
// ID. It is an interface so the webhook handler can be tested without a live
// Nomad cluster.
type Dispatcher interface {
	Dispatch(jobID string) (string, error)
}

// nomadDispatcher dispatches jobs against a real Nomad cluster.
type nomadDispatcher struct {
	client *nomad.Client
	token  string
}

func (d *nomadDispatcher) Dispatch(jobID string) (string, error) {
	job, _, err := d.client.Jobs().Dispatch(jobID, nil, nil, "", &nomad.WriteOptions{
		AuthToken: d.token,
	})
	if err != nil {
		return "", err
	}
	return job.DispatchedJobID, nil
}

// Server holds the dependencies shared across the HTTP handlers.
type Server struct {
	log        *logrus.Entry
	dispatcher Dispatcher
	tracker    *dispatchTracker
}

func RunServer(ctx context.Context, opts *Options) error {
	if opts.Log == nil {
		opts.Log = logrus.WithField("component", "api-server")
	} else {
		opts.Log = opts.Log.WithField("component", "api-server")
	}

	client, err := nomad.NewClient(&nomad.Config{Address: opts.NomadAddress})
	if err != nil {
		return fmt.Errorf("unable to initialize nomad client: %w", err)
	}

	s := &Server{
		log:        opts.Log,
		dispatcher: &nomadDispatcher{client: client, token: opts.NomadToken},
		tracker:    newDispatchTracker(opts.DispatchCooldown),
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Path("/").HandlerFunc(s.RootHandler)
	router.Path("/healthz").HandlerFunc(s.HealthHandler)
	router.Path("/metrics").Handler(promhttp.Handler())
	router.Path("/webhook").Methods(http.MethodPost).HandlerFunc(s.WebhookHandler)

	// Below this point is where the server is started and graceful shutdown occurs.

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", opts.Port),
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			opts.Log.Fatalf("listen: %s\n", err)
		}
	}()
	opts.Log.WithField("port", opts.Port).
		WithField("dispatch_cooldown", opts.DispatchCooldown.String()).
		Info("starting api server")

	<-ctx.Done()

	opts.Log.Info("shutting down api server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		opts.Log.WithError(err).Error("unable to shutdown the api server gracefully")
		return err
	}

	return nil
}
