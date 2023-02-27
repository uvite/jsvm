package v1

import (
	"context"

	"github.com/uvite/jsvm/execution"
	"github.com/uvite/jsvm/lib"
	"github.com/uvite/jsvm/metrics"
	"github.com/uvite/jsvm/metrics/engine"
)

// ControlSurface includes the methods the REST API can use to control and
// communicate with the rest of k6.
type ControlSurface struct {
	RunCtx        context.Context
	Samples       chan metrics.SampleContainer
	MetricsEngine *engine.MetricsEngine
	Scheduler     *execution.Scheduler
	RunState      *lib.TestRunState
}
