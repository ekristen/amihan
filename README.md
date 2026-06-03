# amihan

Trigger Nomad Jobs from Alertmanager Webhook Calls.

`amihan` runs an HTTP API server that receives [Alertmanager webhook][am-webhook]
payloads and dispatches a [parameterized Nomad job][nomad-dispatch] for each
firing alert. It is intended to run inside the cluster on the path
`alertmanager -> amihan -> nomad`.

## How it works

For every alert in the webhook payload, amihan looks for a `nomad_job`
annotation and, if the alert is **firing**, dispatches the named Nomad job:

- Only `firing` alerts dispatch; `resolved` (and any other status) are ignored.
- Repeated deliveries for the same alert are de-duplicated by the alert
  fingerprint, so a job is not re-triggered while its alert keeps firing. A job
  becomes eligible again once the dispatch cooldown elapses (default 5m).
- A failed dispatch does not consume the cooldown, so the next delivery retries.

The endpoint responds with a JSON array describing the outcome per alert
(`dispatched`, `skipped`, or `error`).

## Running

```sh
amihan api-server
```

### Configuration

All flags can be set via environment variables (useful in the Helm chart).

| Flag                  | Env var             | Default                  | Description                                                        |
| --------------------- | ------------------- | ------------------------ | ------------------------------------------------------------------ |
| `--port`, `-p`        | `PORT`              | `4242`                   | Port the API server listens on.                                    |
| `--nomad-address`     | `NOMAD_ADDRESS`     | `http://localhost:4646`  | HTTP URL of the Nomad server.                                      |
| `--nomad-token`       | `NOMAD_TOKEN`       | _(none)_                 | ACL token used to authenticate to Nomad.                           |
| `--dispatch-cooldown` | `DISPATCH_COOLDOWN` | `5m`                     | Minimum time before the same firing alert can re-trigger its job.  |

## Endpoints

| Path        | Method | Description                                                    |
| ----------- | ------ | -------------------------------------------------------------- |
| `/`         | GET    | Returns the app name and version.                              |
| `/healthz`  | GET    | Liveness/readiness probe.                                      |
| `/metrics`  | GET    | Prometheus metrics (includes `amihan_webhook_dispatch_total`). |
| `/webhook`  | POST   | Alertmanager webhook receiver.                                 |

## Alertmanager configuration

Point a receiver at amihan and annotate the alerting rule with the Nomad job to
dispatch.

```yaml
# alertmanager.yml
receivers:
  - name: amihan
    webhook_configs:
      - url: http://amihan/webhook
```

```yaml
# prometheus alerting rule
- alert: ExampleJobTrigger
  expr: some_condition > 0
  annotations:
    nomad_job: parameterized-job
```

[am-webhook]: https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
[nomad-dispatch]: https://developer.hashicorp.com/nomad/docs/job-specification/parameterized
