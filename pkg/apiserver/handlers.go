package apiserver

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/nomad/api"
	"github.com/prometheus/alertmanager/template"
	"github.com/sirupsen/logrus"
	"net/http"

	"github.com/ekristen/amihan/pkg/common"
)

func RootHandler(w http.ResponseWriter, r *http.Request) {
	data := fmt.Sprintf(`{"name":"%s","version":"%s"}`, common.AppVersion.Name, common.AppVersion.Summary)

	w.WriteHeader(200)
	w.Write([]byte(data))
	return
}

func WebhookHandlerWrapper(addr, token string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logrus.WithField("component", "api-server").WithField("handler", "WebhookHandler")
		var data template.Data

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			log.WithError(err).Warn("unable to decode webhook json")
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		for _, alert := range data.Alerts {
			if alert.Annotations == nil {
				continue
			}

			if jobID, ok := alert.Annotations["nomad_job"]; ok {
				log.WithField("jid", jobID).Info("webhook with nomad job found")

				client, err := api.NewClient(&api.Config{
					Address: addr,
				})
				if err != nil {
					log.WithError(err).Error("unable to initialize nomad client")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, err.Error())
					return
				}

				job, _, err := client.Jobs().Dispatch(jobID, nil, nil, "", &api.WriteOptions{
					AuthToken: token,
				})
				if err != nil {
					log.WithError(err).WithField("jid", jobID).Error("unable to dispatch job")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, err.Error())
					return
				}

				log.WithField("djid", job.DispatchedJobID).Info("job dispatched")

				w.Write([]byte(job.DispatchedJobID))
			}
		}
	}

}
