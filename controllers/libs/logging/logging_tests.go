package logging

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type testSink struct {
	lastMsg string
	level   int
	lastErr error
	name    string
}

func (s *testSink) Init(info logr.RuntimeInfo) {}

func (s *testSink) Enabled(level int) bool {
	return true
}

func (s *testSink) Info(level int, msg string, keysAndValues ...interface{}) {
	s.level = level
	s.lastMsg = msg
}

func (s *testSink) Error(err error, msg string, keysAndValues ...interface{}) {
	s.lastErr = err
	s.lastMsg = msg
}

func (s *testSink) WithName(name string) logr.LogSink {
	// Devuelve una nueva instancia o la misma con nombre guardado (para simplificar)
	return &testSink{name: name}
}

func (s *testSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	// Ignoramos para test, solo retornamos la misma instancia
	return s
}

func TestLogFunctions(t *testing.T) {
	sink := &testSink{}
	ctrl.SetLogger(logr.New(sink))

	// Test LogD with no level argument (should default to 1)
	LogD("debug message")
	if sink.lastMsg != "debug message" || sink.level != 1 {
		t.Fatalf("LogD failed: got msg=%q level=%d",
			sink.lastMsg, sink.level)
	}

	// Test LogW (warning message)
	LogW("something might be wrong")
	expectedW := "Warning: something might be wrong"
	if sink.lastMsg != expectedW || sink.level != 0 {
		t.Fatalf("LogW failed: got msg=%q level=%d",
			sink.lastMsg, sink.level)
	}

	// Test LogDeprecation (deprecation warning)
	LogDeprecation("this will be removed soon")
	expectedDep := "Deprecation Warning: this will be removed soon"
	if sink.lastMsg != expectedDep || sink.level != 0 {
		t.Fatalf("LogDeprecation failed: got msg=%q level=%d",
			sink.lastMsg, sink.level)
	}

	// LogTrace logs a message at the TRACE (high verbosity) log level (5).
	LogTrace("debug default")
	if sink.lastMsg != "debug default" || sink.level != 5 {
		t.Fatalf("LogTrace failed: got msg=%q level=%d",
			sink.lastMsg, sink.level)
	}

	// Test LogI (info level 0)
	LogI("info message")
	if sink.lastMsg != "info message" || sink.level != 0 {
		t.Fatalf("LogI failed: got msg=%q level=%d",
			sink.lastMsg, sink.level)
	}

	// Test LogE (error logging with error argument)
	testErr := errors.New("test error")
	LogE(testErr, "error message")
	if sink.lastMsg != "error message" || sink.lastErr != testErr {
		t.Fatalf("LogE failed: got msg=%q err=%v",
			sink.lastMsg, sink.lastErr)
	}
}
