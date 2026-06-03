package apiserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type fakeDispatcher struct {
	calls []string
	err   error
}

func (f *fakeDispatcher) Dispatch(jobID string) (string, error) {
	f.calls = append(f.calls, jobID)
	if f.err != nil {
		return "", f.err
	}
	return jobID + "/dispatch-1", nil
}

func newTestServer(d Dispatcher, cooldown time.Duration) *Server {
	logger := logrus.New()
	logger.SetOutput(os.NewFile(0, os.DevNull))
	return &Server{
		log:        logrus.NewEntry(logger),
		dispatcher: d,
		tracker:    newDispatchTracker(cooldown),
	}
}

func postWebhook(t *testing.T, s *Server, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	s.WebhookHandler(rr, req)
	return rr
}

func TestWebhookHandler_DispatchesFiringJob(t *testing.T) {
	body, err := os.ReadFile("testdata/webhook.json")
	require.NoError(t, err)

	fake := &fakeDispatcher{}
	s := newTestServer(fake, time.Minute)

	rr := postWebhook(t, s, body)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, []string{"parameterized-job"}, fake.calls)

	var results []dispatchResult
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &results))
	require.Len(t, results, 1)
	require.Equal(t, "dispatched", results[0].Result)
}

func TestWebhookHandler_SkipsWithinCooldown(t *testing.T) {
	body, err := os.ReadFile("testdata/webhook.json")
	require.NoError(t, err)

	fake := &fakeDispatcher{}
	s := newTestServer(fake, time.Minute)

	for i := 0; i < 3; i++ {
		rr := postWebhook(t, s, body)
		require.Equal(t, http.StatusOK, rr.Code)
	}

	// Three identical deliveries, but the job is dispatched only once.
	require.Equal(t, []string{"parameterized-job"}, fake.calls)
}

func TestWebhookHandler_IgnoresResolvedAlerts(t *testing.T) {
	body := []byte(`{"alerts":[{"status":"resolved","annotations":{"nomad_job":"some-job"}}]}`)

	fake := &fakeDispatcher{}
	s := newTestServer(fake, time.Minute)

	rr := postWebhook(t, s, body)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Empty(t, fake.calls)
}

func TestWebhookHandler_BadBodyReturns400(t *testing.T) {
	fake := &fakeDispatcher{}
	s := newTestServer(fake, time.Minute)

	rr := postWebhook(t, s, []byte("not json"))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Empty(t, fake.calls)
}

func TestWebhookHandler_DispatchErrorReturns500AndAllowsRetry(t *testing.T) {
	body, err := os.ReadFile("testdata/webhook.json")
	require.NoError(t, err)

	fake := &fakeDispatcher{err: errors.New("nomad down")}
	s := newTestServer(fake, time.Minute)

	rr := postWebhook(t, s, body)
	require.Equal(t, http.StatusInternalServerError, rr.Code)

	// A failed dispatch must not consume the cooldown, so the next delivery retries.
	rr = postWebhook(t, s, body)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Len(t, fake.calls, 2)
}
