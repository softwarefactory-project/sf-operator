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

	testErr := errors.New("test error")

	testCases := []struct {
		name          string
		logFunc       func()
		expectedMsg   string
		expectedLevel int
		expectedErr   error
	}{
		{
			name:          "LogD with default level",
			logFunc:       func() { LogD("debug message") },
			expectedMsg:   "debug message",
			expectedLevel: 1,
		},
		{
			name:          "LogW for warning",
			logFunc:       func() { LogW("something might be wrong") },
			expectedMsg:   "Warning: something might be wrong",
			expectedLevel: 0,
		},
		{
			name:          "LogDeprecation for deprecation warning",
			logFunc:       func() { LogDeprecation("this will be removed soon") },
			expectedMsg:   "Deprecation Warning: this will be removed soon",
			expectedLevel: 0,
		},
		{
			name:          "LogTrace for high verbosity",
			logFunc:       func() { LogTrace("debug default") },
			expectedMsg:   "debug default",
			expectedLevel: 5,
		},
		{
			name:          "LogI for info level",
			logFunc:       func() { LogI("info message") },
			expectedMsg:   "info message",
			expectedLevel: 0,
		},
		{
			name:        "LogE for error",
			logFunc:     func() { LogE(testErr, "error message") },
			expectedMsg: "error message",
			expectedErr: testErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset sink state for each test case
			sink.lastMsg = ""
			sink.level = 0
			sink.lastErr = nil

			tc.logFunc()

			if sink.lastMsg != tc.expectedMsg {
				t.Errorf("Expected message %q, got %q", tc.expectedMsg, sink.lastMsg)
			}
			if sink.lastErr != tc.expectedErr {
				t.Errorf("Expected error %v, got %v", tc.expectedErr, sink.lastErr)
			}
			// Only check level for non-error logs
			if tc.expectedErr == nil && sink.level != tc.expectedLevel {
				t.Errorf("Expected level %d, got %d", tc.expectedLevel, sink.level)
			}
		})
	}
}
