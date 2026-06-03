package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/alertmanager/template"

	"github.com/ekristen/amihan/pkg/common"
)

func (s *Server) RootHandler(w http.ResponseWriter, r *http.Request) {
	data := fmt.Sprintf(`{"name":"%s","version":"%s"}`, common.AppVersion.Name, common.AppVersion.Summary)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(data))
}

// HealthHandler serves the liveness/readiness probe endpoint.
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// dispatchResult is the per-alert outcome reported back to the webhook caller.
type dispatchResult struct {
	Fingerprint     string `json:"fingerprint,omitempty"`
	NomadJob        string `json:"nomad_job"`
	DispatchedJobID string `json:"dispatched_job_id,omitempty"`
	Result          string `json:"result"` // dispatched | skipped | error
	Message         string `json:"message,omitempty"`
}

// WebhookHandler receives an Alertmanager webhook payload and dispatches the
// Nomad job named in each firing alert's "nomad_job" annotation. Repeated
// deliveries for the same alert are de-duplicated via the dispatch cooldown so a
// job is not re-triggered while its alert keeps firing.
func (s *Server) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	log := s.log.WithField("handler", "webhook")

	var data template.Data
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.WithError(err).Warn("unable to decode webhook json")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	results := make([]dispatchResult, 0, len(data.Alerts))
	hadError := false

	for _, alert := range data.Alerts {
		jobID, ok := alert.Annotations["nomad_job"]
		if !ok || jobID == "" {
			continue
		}

		alog := log.WithField("jid", jobID).WithField("fingerprint", alert.Fingerprint)

		// Only dispatch on firing alerts; ignore resolved or other states.
		if alert.Status != "firing" {
			alog.WithField("status", alert.Status).Debug("ignoring non-firing alert")
			continue
		}

		// De-duplicate repeated deliveries for the same firing alert.
		key := dedupKey(alert)
		if allowed, remaining := s.tracker.allow(key, time.Now()); !allowed {
			alog.WithField("cooldown_remaining", remaining.Round(time.Second).String()).
				Info("skipping dispatch, still within cooldown")
			dispatchTotal.WithLabelValues("skipped").Inc()
			results = append(results, dispatchResult{
				Fingerprint: alert.Fingerprint,
				NomadJob:    jobID,
				Result:      "skipped",
				Message:     fmt.Sprintf("within cooldown, %s remaining", remaining.Round(time.Second)),
			})
			continue
		}

		alog.Info("dispatching nomad job")

		dispatchedID, err := s.dispatcher.Dispatch(jobID)
		if err != nil {
			alog.WithError(err).Error("unable to dispatch job")
			dispatchTotal.WithLabelValues("error").Inc()
			hadError = true
			// Allow a prompt retry on the next delivery rather than blocking for
			// the full cooldown after a failed dispatch.
			s.tracker.forget(key)
			results = append(results, dispatchResult{
				Fingerprint: alert.Fingerprint,
				NomadJob:    jobID,
				Result:      "error",
				Message:     "failed to dispatch job",
			})
			continue
		}

		alog.WithField("djid", dispatchedID).Info("job dispatched")
		dispatchTotal.WithLabelValues("dispatched").Inc()
		results = append(results, dispatchResult{
			Fingerprint:     alert.Fingerprint,
			NomadJob:        jobID,
			DispatchedJobID: dispatchedID,
			Result:          "dispatched",
		})
	}

	status := http.StatusOK
	if hadError {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, results)
}

// dedupKey identifies a unique firing alert. Alertmanager populates a stable
// fingerprint per alert; when it is absent we fall back to the job name.
func dedupKey(alert template.Alert) string {
	if alert.Fingerprint != "" {
		return alert.Fingerprint
	}
	return "job:" + alert.Annotations["nomad_job"]
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
