package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestGenError(t *testing.T) {
	err := errors.New("original error")
	desc := "description"
	wrapped := genError(desc, err)
	assert.Equal(t, fmt.Sprintf("%s: %s", desc, err), wrapped.Error())
	assert.True(t, errors.Is(wrapped, err))
}

func TestLogError(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.ErrorLevel)
	observedLogger := zap.New(observedZapCore)

	err := errors.New("test error")
	desc := "test description"
	logError(observedLogger, desc, err)

	assert.Equal(t, 1, observedLogs.Len())
	logEntry := observedLogs.All()[0]
	assert.Equal(t, desc, logEntry.Message)
	// The observer might store the error as a string or the error itself depending on zap version/config
	assert.Contains(t, fmt.Sprint(logEntry.ContextMap()["error"]), "test error")
}

func TestLogErrors(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.ErrorLevel)
	observedLogger := zap.New(observedZapCore)

	errs := []error{errors.New("error 1"), errors.New("error 2")}
	desc := "test description"
	logErrors(observedLogger, desc, errs)

	assert.Equal(t, 2, observedLogs.Len())
	for i, logEntry := range observedLogs.All() {
		assert.Equal(t, desc, logEntry.Message)
		assert.Contains(t, fmt.Sprint(logEntry.ContextMap()["error"]), fmt.Sprintf("error %d", i+1))
	}
}
