package lsp

// Test helpers shared across test files

// testLogger is a no-op logger for tests
type testLogger struct{}

func (l *testLogger) Debugf(format string, args ...interface{}) {}
func (l *testLogger) Infof(format string, args ...interface{})  {}
func (l *testLogger) Warnf(format string, args ...interface{})  {}
func (l *testLogger) Errorf(format string, args ...interface{}) {}
func (l *testLogger) Fatalf(format string, args ...interface{}) {}
