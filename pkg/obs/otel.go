// Package obs initializes OpenTelemetry tracing+metrics, exporting to Cloud
// Trace / Cloud Monitoring. Matches Scion's OTEL conventions so spans
// correlate end-to-end (chat → ingress → router → Scion → emitter).
package obs

import "context"

type Shutdown func(context.Context) error

// Init wires up OTEL providers. TODO: import
//   go.opentelemetry.io/otel
//   github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace
func Init(ctx context.Context, serviceName string) (Shutdown, error) {
	return func(context.Context) error { return nil }, nil
}
