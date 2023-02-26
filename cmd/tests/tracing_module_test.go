package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd"
	"go.k6.io/k6/lib/testutils/httpmultibin"
)

func TestTracingModuleClient(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)
		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		const instrumentedHTTP = new tracing.Client({
			propagator: "w3c",
		})

		export default async function () {
			instrumentedHTTP.del("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.get("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.head("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.options("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.patch("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.post("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.put("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.request("GET", "HTTPBIN_IP_URL/tracing");
            await instrumentedHTTP.asyncRequest("GET", "HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(9), atomic.LoadInt64(&gotRequests))

	jsonResults, err := afero.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	assertHasTraceIDMetadata(t, jsonResults)
}

func TestTracingClient_DoesNotInterfereWithHTTPModule(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64
	var gotInstrumentedRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)

		if r.Header.Get("traceparent") != "" {
			atomic.AddInt64(&gotInstrumentedRequests, 1)
			assert.Len(t, r.Header.Get("traceparent"), 55)
		}
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		const instrumentedHTTP = new tracing.Client({
			propagator: "w3c",
		})

		export default function () {
			instrumentedHTTP.get("HTTPBIN_IP_URL/tracing");
			http.get("HTTPBIN_IP_URL/tracing");
			instrumentedHTTP.head("HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(3), atomic.LoadInt64(&gotRequests))
	assert.Equal(t, int64(2), atomic.LoadInt64(&gotInstrumentedRequests))
}

func TestTracingInstrumentHTTP_W3C(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)
		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import tracing from "k6/experimental/tracing";

		tracing.instrumentHTTP({
			propagator: "w3c",
		})

		export default async function () {
			http.del("HTTPBIN_IP_URL/tracing");
			http.get("HTTPBIN_IP_URL/tracing");
			http.head("HTTPBIN_IP_URL/tracing");
			http.options("HTTPBIN_IP_URL/tracing");
			http.patch("HTTPBIN_IP_URL/tracing");
			http.post("HTTPBIN_IP_URL/tracing");
			http.put("HTTPBIN_IP_URL/tracing");
			http.request("GET", "HTTPBIN_IP_URL/tracing");
			await http.asyncRequest("GET", "HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(9), atomic.LoadInt64(&gotRequests))

	jsonResults, err := afero.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	assertHasTraceIDMetadata(t, jsonResults)
}

func TestTracingInstrumentHTTP_Jaeger(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)
		assert.NotEmpty(t, r.Header.Get("uber-trace-id"))
		assert.Len(t, r.Header.Get("uber-trace-id"), 45)
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import { check } from "k6";
		import tracing from "k6/experimental/tracing";

		tracing.instrumentHTTP({
			propagator: "jaeger",
		})

		export default function () {
			http.del("HTTPBIN_IP_URL/tracing");
			http.get("HTTPBIN_IP_URL/tracing");
			http.head("HTTPBIN_IP_URL/tracing");
			http.options("HTTPBIN_IP_URL/tracing");
			http.patch("HTTPBIN_IP_URL/tracing");
			http.post("HTTPBIN_IP_URL/tracing");
			http.put("HTTPBIN_IP_URL/tracing");
			http.request("GET", "HTTPBIN_IP_URL/tracing");
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(8), atomic.LoadInt64(&gotRequests))

	jsonResults, err := afero.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	assertHasTraceIDMetadata(t, jsonResults)
}

func TestTracingInstrumentHTTP_FillsParams(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	var gotRequests int64

	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)

		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)

		assert.NotEmpty(t, r.Header.Get("X-Test-Header"))
		assert.Equal(t, "test", r.Header.Get("X-Test-Header"))
	})

	script := tb.Replacer.Replace(`
		import http from "k6/http";
		import tracing from "k6/experimental/tracing";

		tracing.instrumentHTTP({
			propagator: "w3c",
		})

		const testHeaders = {
			"X-Test-Header": "test",
		}

		export default function () {
			http.del("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.get("HTTPBIN_IP_URL/tracing", { headers: testHeaders });
			http.head("HTTPBIN_IP_URL/tracing", { headers: testHeaders });
			http.options("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.patch("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.post("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.put("HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
			http.request("GET", "HTTPBIN_IP_URL/tracing", null, { headers: testHeaders });
		};
	`)

	ts := getSingleFileTestState(t, script, []string{"--out", "json=results.json"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Equal(t, int64(8), atomic.LoadInt64(&gotRequests))

	jsonResults, err := afero.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	assertHasTraceIDMetadata(t, jsonResults)
}

func TestTracingInstrummentHTTP_SupportsMultipleTestScripts(t *testing.T) {
	t.Parallel()

	var gotRequests int64

	tb := httpmultibin.NewHTTPMultiBin(t)
	tb.Mux.HandleFunc("/tracing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&gotRequests, 1)

		assert.NotEmpty(t, r.Header.Get("traceparent"))
		assert.Len(t, r.Header.Get("traceparent"), 55)
	})

	mainScript := tb.Replacer.Replace(`
		import http from "k6/http";
		import tracing from "k6/experimental/tracing";

		import { iShouldBeInstrumented } from "./imported.js";
		
		tracing.instrumentHTTP({
			propagator: "w3c",
		})

		export default function() {
			iShouldBeInstrumented();
		};
	`)

	importedScript := tb.Replacer.Replace(`
		import http from "k6/http";

		export function iShouldBeInstrumented() {
			http.head("HTTPBIN_IP_URL/tracing");
		}
	`)

	ts := NewGlobalTestState(t)
	require.NoError(t, afero.WriteFile(ts.FS, filepath.Join(ts.Cwd, "main.js"), []byte(mainScript), 0o644))
	require.NoError(t, afero.WriteFile(ts.FS, filepath.Join(ts.Cwd, "imported.js"), []byte(importedScript), 0o644))

	ts.CmdArgs = []string{"k6", "run", "--out", "json=results.json", "main.js"}
	ts.ExpectedExitCode = 0

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	jsonResults, err := afero.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)

	assert.Equal(t, int64(1), atomic.LoadInt64(&gotRequests))
	assertHasTraceIDMetadata(t, jsonResults)
}

// assertHasTraceIDMetadata checks that the trace_id metadata is present and has the correct format
// for all http metrics in the json results file.
func assertHasTraceIDMetadata(t *testing.T, jsonResults []byte) {
	gotHTTPDataPoints := false

	for _, jsonLine := range bytes.Split(jsonResults, []byte("\n")) {
		if len(jsonLine) == 0 {
			continue
		}

		var line sampleEnvelope
		require.NoError(t, json.Unmarshal(jsonLine, &line))

		if line.Type != "Point" {
			continue
		}

		// Filter metric samples which are not related to http
		if !strings.HasPrefix(line.Metric, "http_") {
			continue
		}

		gotHTTPDataPoints = true

		anyTraceID, hasTraceID := line.Data.Metadata["trace_id"]
		require.True(t, hasTraceID)

		traceID, gotTraceID := anyTraceID.(string)
		require.True(t, gotTraceID)

		assert.Len(t, traceID, 32)
	}

	assert.True(t, gotHTTPDataPoints)
}

// sampleEnvelope is a trimmed version of the struct found
// in output/json/wrapper.go
// TODO: use the json output's wrapper struct instead if it's ever exported
type sampleEnvelope struct {
	Metric string `json:"metric"`
	Type   string `json:"type"`
	Data   struct {
		Value    float64                `json:"value"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"data"`
}
