package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/freygold/tasker/internal/config"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/newrelic/go-agent/v3/newrelic/integrationsupport"
	"github.com/rs/zerolog"
)

func TestLocalLogsAreForwarded(t *testing.T) {
	app := integrationsupport.NewTestApp(
		integrationsupport.SampleEverythingReplyFn,
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	service := &LoggerService{nrApp: app.Application}
	cfg := &config.ObservabilityConfig{
		ServiceName: "tasker",
		Environment: "local",
		Logging:     config.LoggingConfig{Level: "info", Format: "console"},
		NewRelic:    config.NewRelicConfig{AppLogForwardingEnabled: true},
	}

	logger := NewLoggerWithService(cfg, service)
	logger.Info().Msg("request handled")

	// New Relic's test helper uses an internal type for expected logs.
	method := reflect.ValueOf(app.ExpectLogEvents)
	expected := reflect.MakeSlice(method.Type().In(1), 1, 1)
	event := expected.Index(0)
	event.FieldByName("Message").SetString("xxANY-STRINGxx")
	event.FieldByName("Severity").SetString("info")
	event.FieldByName("Timestamp").SetInt(-1)
	method.Call([]reflect.Value{reflect.ValueOf(t), expected})
}

func TestLocalLogsRespectDisabledForwarding(t *testing.T) {
	app := integrationsupport.NewTestApp(
		integrationsupport.SampleEverythingReplyFn,
		newrelic.ConfigAppLogForwardingEnabled(false),
	)
	service := &LoggerService{nrApp: app.Application}
	cfg := &config.ObservabilityConfig{
		ServiceName: "tasker",
		Environment: "local",
		Logging:     config.LoggingConfig{Level: "info", Format: "console"},
	}

	logger := NewLoggerWithService(cfg, service)
	logger.Info().Msg("request handled")
	app.ExpectLogEvents(t, nil)
}

func TestTraceContextAddsIDsToLogs(t *testing.T) {
	app := integrationsupport.NewTestApp(
		integrationsupport.SampleEverythingReplyFn,
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	txn := app.StartTransaction("GET /status")
	defer txn.End()
	metadata := txn.GetTraceMetadata()
	if metadata.TraceID == "" || metadata.SpanID == "" {
		t.Fatal("transaction has no trace or span ID")
	}

	var output bytes.Buffer
	logger := WithTraceContext(zerolog.New(&output), txn)
	logger.Info().Msg("request handled")
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["trace.id"] != metadata.TraceID || entry["span.id"] != metadata.SpanID {
		t.Errorf("log trace context = %v, want trace %s and span %s", entry, metadata.TraceID, metadata.SpanID)
	}
}

func TestInvalidNewRelicConfigurationIsReported(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	previousStderr := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = previousStderr }()

	service := NewLoggerService(&config.ObservabilityConfig{
		ServiceName: "tasker",
		NewRelic:    config.NewRelicConfig{LicenseKey: "invalid"},
	})
	os.Stderr = previousStderr
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if service.GetApplication() != nil {
		t.Error("New Relic started with an invalid license")
	}
	if !strings.Contains(string(output), "New Relic could not start") {
		t.Error("New Relic did not report the configuration error")
	}
}
