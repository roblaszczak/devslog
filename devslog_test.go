package humanslog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewHandler(t *testing.T) {
	testNewHandlerDefaults(t)
	testNewHandlerWithOptions(t)
	testNewHandlerWithNilOptions(t)
	testNewHandlerWithNilSlogHandlerOptions(t)
}

func TestMethods(t *testing.T) {
	testEnabled(t)
	testWithGroup(t)
	testWithGroupEmpty(t)
	testWithAttrs(t)
	testWithAttrsEmpty(t)
}

func TestLevels(t *testing.T) {
	testLevelMessageDebug(t)
	testLevelMessageInfo(t)
	testLevelMessageWarn(t)
	testLevelMessageError(t)
}

func TestGroupsAndAttributes(t *testing.T) {
	testWithGroups(t)
	testWithGroupsEmpty(t)
	testWithAttributes(t)
	testWithAttributesRaceCondition()
}

func TestSourceAndReplace(t *testing.T) {
	testSource(t)
	testReplaceLevelAttributes(t)
}

func TestTypes(t *testing.T) {
	slogOpts := &slog.HandlerOptions{
		AddSource:   false,
		Level:       slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { return a },
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
		StringerFormatter: true,
	}

	testString(t, opts)
	testIntFloat(t, opts)
	testBool(t, opts)
	testTime(t, opts)
	testError(t, opts)
	testSlice(t, opts)
	testSliceBig(t, opts)
	testMap(t, opts)
	testMapOfPointers(t, opts)
	testMapOfInterface(t, opts)
	testStruct(t, opts)
	testNilInterface(t, opts)
	testGroup(t, opts)
	testLogValuer(t, opts)
	testLogValuerPanic(t, opts)
	testStringer(t, opts)
	testStringerInner(t, opts)
	testNoColor(t, opts)
	testInfinite(t, opts)
	testSameSourceInfoColor(t)
}

func testNewHandlerDefaults(t *testing.T) {
	opts := &Options{
		HandlerOptions: &slog.HandlerOptions{},
	}
	h := NewHandler(os.Stdout, opts)

	if h.opts.Level.Level() != slog.LevelInfo.Level() {
		t.Errorf("Expected default log level to be LevelInfo")
	}

	if h.opts.MaxSlicePrintSize != 50 {
		t.Errorf("Expected default MaxSlicePrintSize to be 50")
	}

	if h.opts.TimeFormat != "[15:04:05]" {
		t.Errorf("Expected default TimeFormat to be \"[15:04:05]\" ")
	}

	if h.out == nil {
		t.Errorf("Expected writer to be initialized")
	}
}

func testNewHandlerWithOptions(t *testing.T) {
	handlerOpts := &Options{
		HandlerOptions:    &slog.HandlerOptions{Level: slog.LevelWarn},
		MaxSlicePrintSize: 10,
		TimeFormat:        "[04:05]",
	}
	h := NewHandler(nil, handlerOpts)

	if h.opts.Level.Level() != slog.LevelWarn.Level() {
		t.Errorf("Expected custom log level to be LevelWarn")
	}

	if h.opts.MaxSlicePrintSize != 10 {
		t.Errorf("Expected custom MaxSlicePrintSize to be 10")
	}

	if h.opts.TimeFormat != "[04:05]" {
		t.Errorf("Expected custom TimeFormat to be \"[04:05]\" ")
	}
}

func testNewHandlerWithNilOptions(t *testing.T) {
	h := NewHandler(nil, nil)

	if h.opts.HandlerOptions == nil || h.opts.Level != slog.LevelInfo {
		t.Errorf("Expected HandlerOptions to be initialized with default level")
	}

	if h.opts.MaxSlicePrintSize != 50 {
		t.Errorf("Expected MaxSlicePrintSize to be initialized with default value")
	}

	if h.out != nil {
		t.Errorf("Expected writer to be nil")
	}
}

func testNewHandlerWithNilSlogHandlerOptions(t *testing.T) {
	opts := &Options{}
	h := NewHandler(nil, opts)

	if h.opts.HandlerOptions == nil || h.opts.Level != slog.LevelInfo {
		t.Errorf("Expected HandlerOptions to be initialized with default level")
	}

	if h.opts.MaxSlicePrintSize != 50 {
		t.Errorf("Expected MaxSlicePrintSize to be initialized with default value")
	}

	if h.out != nil {
		t.Errorf("Expected writer to be nil")
	}
}

func testEnabled(t *testing.T) {
	h := NewHandler(nil, nil)
	ctx := context.Background()

	if !h.Enabled(ctx, slog.LevelInfo) {
		t.Error("Expected handler to be enabled for LevelInfo")
	}

	if h.Enabled(ctx, slog.LevelDebug) {
		t.Error("Expected handler to be disabled for LevelDebug")
	}
}

func testWithGroup(t *testing.T) {
	h := NewHandler(nil, nil)
	h2 := h.WithGroup("myGroup")

	if h2 == h {
		t.Error("Expected a new handler instance")
	}
}

func testWithGroupEmpty(t *testing.T) {
	h := NewHandler(nil, nil)
	h2 := h.WithGroup("")

	if h2 != h {
		t.Error("Expected a original handler instance")
	}
}

func testWithAttrs(t *testing.T) {
	h := NewHandler(nil, nil)
	h2 := h.WithAttrs([]slog.Attr{slog.String("key", "value")})

	if h2 == h {
		t.Error("Expected a new handler instance")
	}
}

func testWithAttrsEmpty(t *testing.T) {
	h := NewHandler(nil, nil)
	h2 := h.WithAttrs([]slog.Attr{})

	if h2 != h {
		t.Error("Expected a original handler instance")
	}
}

func testLevelMessageDebug(t *testing.T) {
	h := NewHandler(nil, nil)
	buf := make([]byte, 0)
	record := &slog.Record{
		Level:   slog.LevelDebug,
		Message: "Debug message",
	}

	buf = h.levelMessage(buf, record)

	expected := "\x1b[44m\x1b[30m DEBUG \x1b[0m \x1b[34mDebug message\x1b[0m\n"
	result := string(buf)

	if result != expected {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, result)
	}
}

func testLevelMessageInfo(t *testing.T) {
	h := NewHandler(nil, nil)
	buf := make([]byte, 0)
	record := &slog.Record{
		Level:   slog.LevelInfo,
		Message: "Info message",
	}

	buf = h.levelMessage(buf, record)

	expected := "\x1b[42m\x1b[30m INFO \x1b[0m \x1b[32mInfo message\x1b[0m\n"
	result := string(buf)

	if result != expected {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, result)
	}
}

func testLevelMessageWarn(t *testing.T) {
	h := NewHandler(nil, nil)
	buf := make([]byte, 0)
	record := &slog.Record{
		Level:   slog.LevelWarn,
		Message: "Warning message",
	}

	buf = h.levelMessage(buf, record)

	expected := "\x1b[43m\x1b[30m WARN \x1b[0m \x1b[33mWarning message\x1b[0m\n"
	result := string(buf)

	if result != expected {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, result)
	}
}

func testLevelMessageError(t *testing.T) {
	h := NewHandler(nil, nil)
	buf := make([]byte, 0)
	record := &slog.Record{
		Level:   slog.LevelError,
		Message: "Error message",
	}

	buf = h.levelMessage(buf, record)

	expected := "\x1b[41m\x1b[30m ERROR \x1b[0m \x1b[31mError message\x1b[0m\n"
	result := string(buf)

	if result != expected {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, result)
	}
}

func (w *MockWriter) Write(p []byte) (int, error) {
	w.WrittenData = append(w.WrittenData, p...)
	return len(p), nil
}

type MockWriter struct {
	WrittenData []byte
}

func testSource(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { return a },
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	_, filename, l, _ := runtime.Caller(0)
	logger.Info("message")

	expected := fmt.Sprintf("\x1b[2m[]\x1b[0m \x1b[37m%s:%d\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m message\n\n", filename, l+1)

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testWithGroups(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
	}

	logger := slog.New(NewHandler(w, opts).WithGroup("test_group"))

	logger.Info("My INFO message",
		slog.Any("a", "1"),
	)

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m My INFO message \x1b[90mtest_group.a=\x1b[0m1\n\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testWithGroupsEmpty(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
	}

	logger := slog.New(NewHandler(w, opts).WithGroup("test_group"))

	logger.Info("My INFO message")

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m My INFO message\n\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testWithAttributes(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
	}

	as := []slog.Attr{slog.Any("a", "1")}
	logger := slog.New(NewHandler(w, opts).WithAttrs(as))

	logger.Info("My INFO message")

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m My INFO message \x1b[90ma=\x1b[0m1\n\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testWithAttributesRaceCondition() {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
	}

	logger := slog.New(NewHandler(w, opts))

	go func() {
		as := []slog.Attr{slog.Any("a", "1")}
		logger.Handler().WithAttrs(as)
	}()

	go func() {
		logger.Info("INFO message")
	}()
}

const (
	LevelTrace     = slog.Level(-8)
	LevelDebug     = slog.LevelDebug
	LevelInfo      = slog.LevelInfo
	LevelNotice    = slog.Level(2)
	LevelWarning   = slog.LevelWarn
	LevelError     = slog.LevelError
	LevelEmergency = slog.Level(12)
)

func testReplaceLevelAttributes(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource:   false,
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceAttributes,
	}

	opts := &Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 4,
		SortKeys:          true,
		TimeFormat:        "[]",
		NewLineAfterLog:   true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	ctx := context.Background()
	logger.Log(ctx, LevelEmergency, "missing pilots")
	logger.Error("failed to start engines", "err", "missing fuel")
	logger.Warn("falling back to default value")
	logger.Log(ctx, LevelNotice, "all systems are running")
	logger.Info("initiating launch")
	logger.Debug("starting background job")
	logger.Log(ctx, LevelTrace, "button clicked")

	expected := "\x1b[2m[]\x1b[0m \x1b[41m\x1b[30m EMERGENCY \x1b[0m missing pilots \x1b[90msev=\x1b[0mEMERGENCY\n\n\x1b[2m[]\x1b[0m \x1b[41m\x1b[30m ERROR \x1b[0m failed to start engines \x1b[90merr=\x1b[0mmissing fuel \x1b[90msev=\x1b[0mERROR\n\n\x1b[2m[]\x1b[0m \x1b[43m\x1b[30m WARNING \x1b[0m falling back to default value \x1b[90msev=\x1b[0mWARNING\n\n\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m NOTICE \x1b[0m all systems are running \x1b[90msev=\x1b[0mNOTICE\n\n\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m initiating launch \x1b[90msev=\x1b[0mINFO\n\n\x1b[2m[]\x1b[0m \x1b[44m\x1b[30m DEBUG \x1b[0m starting background job \x1b[90msev=\x1b[0mDEBUG\n\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func replaceAttributes(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		// Rename the level key from "level" to "sev".
		a.Key = "sev"

		// Handle custom level values.
		level := a.Value.Any().(slog.Level)

		// This could also look up the name from a map or other structure, but
		// this demonstrates using a switch statement to rename levels. For
		// maximum performance, the string values should be constants, but this
		// example uses the raw strings for readability.
		switch {
		case level < LevelDebug:
			a.Value = slog.StringValue("TRACE")
		case level < LevelInfo:
			a.Value = slog.StringValue("DEBUG")
		case level < LevelNotice:
			a.Value = slog.StringValue("INFO")
		case level < LevelWarning:
			a.Value = slog.StringValue("NOTICE")
		case level < LevelError:
			a.Value = slog.StringValue("WARNING")
		case level < LevelEmergency:
			a.Value = slog.StringValue("ERROR")
		default:
			a.Value = slog.StringValue("EMERGENCY")
		}
	}

	return a
}

func testString(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	s := "string"

	logger.Info("msg",
		slog.Any("s", s),
		slog.Any("sp", &s),
		slog.Any("empty", ""),
		slog.Any("url", "https://go.dev/"),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mempty=\x1b[0m \x1b[90ms=\x1b[0mstring \x1b[90msp=\x1b[0m\x1b[31m*\x1b[0mstring \x1b[90murl=\x1b[0m\x1b[36mhttps://go.dev/\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testIntFloat(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	f := 1.21
	fp := &f
	i := 1
	ip := &i
	logger.Info("msg",
		slog.Any("f", f),
		slog.Any("fp", fp),
		slog.Any("i", i),
		slog.Any("ip", ip),
	)

	expected := fmt.Sprintf(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mf=\x1b[0m\x1b[36m1.21\x1b[0m \x1b[90mfp=\x1b[0m\x1b[31m*\x1b[0m\x1b[36m1.21\x1b[0m \x1b[90mi=\x1b[0m\x1b[36m1\x1b[0m \x1b[90mip=\x1b[0m\x1b[31m*\x1b[0m\x1b[36m1\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testBool(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	b := true
	bp := &b
	logger.Info("msg",
		slog.Any("b", b),
		slog.Any("bp", bp),
	)

	expected := fmt.Sprintf("\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mb=\x1b[0m\x1b[32mtrue\x1b[0m \x1b[90mbp=\x1b[0m\x1b[31m*\x1b[0m\x1b[32mtrue\x1b[0m\n\n")

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testTime(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	timeT := time.Date(2012, time.March, 28, 0, 0, 0, 0, time.UTC)
	timeE := time.Date(2023, time.August, 15, 12, 0, 0, 0, time.UTC)
	timeD := timeE.Sub(timeT)

	logger.Info("msg",
		slog.Any("t", timeT),
		slog.Any("tp", &timeT),
		slog.Any("d", timeD),
		slog.Any("tp", &timeD),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90md=\x1b[0m\x1b[37m99780h0m0s\x1b[0m \x1b[90mt=\x1b[0m\x1b[37m2012-03-28 00:00:00 +0000 UTC\x1b[0m \x1b[90mtp=\x1b[0m\x1b[37m2012-03-28 00:00:00 +0000 UTC\x1b[0m \x1b[90mtp=\x1b[0m\x1b[37m99780h0m0s\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testError(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	e := fmt.Errorf("broken")
	e = fmt.Errorf("err 1: %w", e)
	e = fmt.Errorf("err 2: %w", e)

	logger.Info("msg",
		slog.Any("e", e),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90me=\x1b[0m\x1b[31merr 2: err 1: broken\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testSlice(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	s := []string{"apple", "ba na na"}

	logger.Info("msg",
		slog.Any("s", s),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90ms=\x1b[0m\x1b[36m2\x1b[0m \x1b[32m[\x1b[0m\x1b[32m]\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mg\x1b[0m\x1b[32m{\x1b[0mapple ba na na\x1b[32m}\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testSliceBig(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	s := make([]int, 0)
	for i := 0; i < 11; i++ {
		s = append(s, i*2)
	}

	logger.Info("msg",
		slog.Any("s", s),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90ms=\x1b[0m\x1b[36m11\x1b[0m \x1b[32m[\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m{\x1b[0m\x1b[36m0\x1b[0m \x1b[36m2\x1b[0m \x1b[36m4\x1b[0m \x1b[36m6\x1b[0m \x1b[36m...\x1b[0m\x1b[32m}\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testMap(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	m := map[int]string{0: "a", 1: "b"}
	mp := &m

	logger.Info("msg",
		slog.Any("m", m),
		slog.Any("mp", mp),
		slog.Any("mpp", &mp),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mm=\x1b[0m\x1b[36m2\x1b[0m \x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mg\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=b\x1b[32m}\x1b[0m \x1b[90mmp=\x1b[0m\x1b[31m*\x1b[0m\x1b[36m2\x1b[0m \x1b[31m*\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mg\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=b\x1b[32m}\x1b[0m \x1b[90mmpp=\x1b[0m\x1b[31m*\x1b[0m\x1b[31m*\x1b[0m\x1b[36m2\x1b[0m \x1b[31m*\x1b[0m\x1b[31m*\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mg\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=b\x1b[32m}\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testMapOfPointers(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	s := "a"
	m := map[int]*string{0: &s, 1: &s}

	logger.Info("msg",
		slog.Any("m", m),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mm=\x1b[0m\x1b[36m2\x1b[0m \x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[31m*\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mg\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=a\x1b[32m}\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testMapOfInterface(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	m := map[int]any{0: "a", 1: "b"}
	mp := &m

	logger.Info("msg",
		slog.Any("m", m),
		slog.Any("mp", mp),
		slog.Any("mpp", &mp),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mm=\x1b[0m\x1b[36m2\x1b[0m \x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33mr\x1b[0m\x1b[33mf\x1b[0m\x1b[33ma\x1b[0m\x1b[33mc\x1b[0m\x1b[33me\x1b[0m\x1b[33m \x1b[0m\x1b[33m{\x1b[0m\x1b[33m}\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=b\x1b[32m}\x1b[0m \x1b[90mmp=\x1b[0m\x1b[31m*\x1b[0m\x1b[36m2\x1b[0m \x1b[31m*\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33mr\x1b[0m\x1b[33mf\x1b[0m\x1b[33ma\x1b[0m\x1b[33mc\x1b[0m\x1b[33me\x1b[0m\x1b[33m \x1b[0m\x1b[33m{\x1b[0m\x1b[33m}\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=b\x1b[32m}\x1b[0m \x1b[90mmpp=\x1b[0m\x1b[31m*\x1b[0m\x1b[31m*\x1b[0m\x1b[36m2\x1b[0m \x1b[31m*\x1b[0m\x1b[31m*\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33mr\x1b[0m\x1b[33mf\x1b[0m\x1b[33ma\x1b[0m\x1b[33mc\x1b[0m\x1b[33me\x1b[0m\x1b[33m \x1b[0m\x1b[33m{\x1b[0m\x1b[33m}\x1b[0m\x1b[32m{\x1b[0m\x1b[32m0\x1b[0m=a \x1b[32m1\x1b[0m=b\x1b[32m}\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testStruct(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	type StructTest struct {
		Slice      []int
		Map        map[int]int
		Struct     struct{ B bool }
		SliceP     *[]int
		MapP       *map[int]int
		StructP    *struct{ B bool }
		unexported int
	}

	s := &StructTest{
		Slice:      []int{},
		Map:        map[int]int{},
		Struct:     struct{ B bool }{},
		SliceP:     &[]int{},
		MapP:       &map[int]int{},
		StructP:    &struct{ B bool }{},
		unexported: 5,
	}

	logger.Info("msg",
		slog.Any("s", s),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg\x1b[33mS\x1b[0m \x1b[90ms\x1b[0m=\x1b[31m*\x1b[0m\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33mS\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mu\x1b[0m\x1b[33mc\x1b[0m\x1b[33mt\x1b[0m\x1b[33mT\x1b[0m\x1b[33me\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\n    \x1b[32mSlice\x1b[0m  : \x1b[36m0\x1b[0m \x1b[32m[\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m{\x1b[0m\x1b[32m}\x1b[0m\n    \x1b[32mMap\x1b[0m    : \x1b[36m0\x1b[0m \x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m{\x1b[0m\x1b[32m}\x1b[0m\n    \x1b[32mStruct\x1b[0m : \x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mu\x1b[0m\x1b[33mc\x1b[0m\x1b[33mt\x1b[0m\x1b[33m \x1b[0m\x1b[33m{\x1b[0m\x1b[33m \x1b[0m\x1b[33mB\x1b[0m\x1b[33m \x1b[0m\x1b[33mb\x1b[0m\x1b[33mo\x1b[0m\x1b[33mo\x1b[0m\x1b[33ml\x1b[0m\x1b[33m \x1b[0m\x1b[33m}\x1b[0m\n      \x1b[32mB\x1b[0m: \x1b[31mfalse\x1b[0m\n    \x1b[32mSliceP\x1b[0m : \x1b[36m0\x1b[0m \x1b[31m*\x1b[0m\x1b[32m[\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m{\x1b[0m\x1b[32m}\x1b[0m\n    \x1b[32mMapP\x1b[0m   : \x1b[36m0\x1b[0m \x1b[31m*\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mp\x1b[0m\x1b[32m[\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m]\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[32m{\x1b[0m\x1b[32m}\x1b[0m\n    \x1b[32mStructP\x1b[0m: \x1b[31m*\x1b[0m\x1b[33ms\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mu\x1b[0m\x1b[33mc\x1b[0m\x1b[33mt\x1b[0m\x1b[33m \x1b[0m\x1b[33m{\x1b[0m\x1b[33m \x1b[0m\x1b[33mB\x1b[0m\x1b[33m \x1b[0m\x1b[33mb\x1b[0m\x1b[33mo\x1b[0m\x1b[33mo\x1b[0m\x1b[33ml\x1b[0m\x1b[33m \x1b[0m\x1b[33m}\x1b[0m\n      \x1b[32mB\x1b[0m: \x1b[31mfalse\x1b[0m\n\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testNilInterface(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	type StructWithInterface struct {
		Data any
	}

	s := StructWithInterface{}

	logger.Info("msg",
		slog.Any("s", s),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg\x1b[33mS\x1b[0m \x1b[90ms\x1b[0m=\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33mS\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mu\x1b[0m\x1b[33mc\x1b[0m\x1b[33mt\x1b[0m\x1b[33mW\x1b[0m\x1b[33mi\x1b[0m\x1b[33mt\x1b[0m\x1b[33mh\x1b[0m\x1b[33mI\x1b[0m\x1b[33mn\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33mr\x1b[0m\x1b[33mf\x1b[0m\x1b[33ma\x1b[0m\x1b[33mc\x1b[0m\x1b[33me\x1b[0m\n    \x1b[32mData\x1b[0m: \x1b[33m<nil>\x1b[0m\n\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testGroup(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	logger.Info("msg",
		slog.Any("1", "a"),
		slog.Group("g",
			slog.Any("2", "b"),
		),
	)

	expected := []byte("\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90m1=\x1b[0ma \x1b[90mg.2=\x1b[0mb\n\n")

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

type logValuerExample1 struct {
	A int
	B string
}

func (item logValuerExample1) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("A", item.A),
		slog.String("B", item.B),
	)
}

func testLogValuer(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))
	item1 := logValuerExample1{
		A: 5,
		B: "test",
	}
	logger.Info("test_log_valuer",
		slog.Any("item1", item1),
	)

	expected := []byte("\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test_log_valuer \x1b[90mitem1.A=\x1b[0m\x1b[36m5\x1b[0m \x1b[90mitem1.B=\x1b[0mtest\n\n")

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

type logValuerExample2 struct {
	A int
	B string
}

func (item logValuerExample2) LogValue() slog.Value {
	panic("log valuer paniced")
}

func testLogValuerPanic(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))
	item1 := logValuerExample2{
		A: 5,
		B: "test",
	}
	logger.Info("test_log_valuer_panic",
		slog.Any("item1", item1),
	)

	expectedPrefix := []byte("\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test_log_valuer_panic \x1b[90mitem1=\x1b[0m\x1b[31mLogValue panicked\n")
	if !bytes.HasPrefix(w.WrittenData, expectedPrefix) {
		t.Errorf("\nGot:\n%s\n , %[1]q expected it to contain panic stack trace", w.WrittenData)
	}
}

type logStringerExample1 struct {
	A []byte
}

func (item logStringerExample1) String() string {
	return fmt.Sprintf("A: %s", item.A)
}

func testStringer(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))
	item1 := logStringerExample1{
		A: []byte("test"),
	}
	logger.Info("test_stringer",
		slog.Any("item1", item1),
	)

	expected := []byte("\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test_stringer \x1b[90mitem1=\x1b[0mA: test\n\n")

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

type logStringerExample2 struct {
	Inner logStringerExample1
	Other int
}

func testStringerInner(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))
	item1 := logStringerExample2{
		Inner: logStringerExample1{
			A: []byte("test"),
		},
		Other: 42,
	}
	logger.Info("test_stringer_inner",
		slog.Any("item1", item1),
	)

	expected := []byte(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test_stringer_inner \x1b[90mitem1=\x1b[0m\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33mS\x1b[0m\x1b[33mt\x1b[0m\x1b[33mr\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mg\x1b[0m\x1b[33me\x1b[0m\x1b[33mr\x1b[0m\x1b[33mE\x1b[0m\x1b[33mx\x1b[0m\x1b[33ma\x1b[0m\x1b[33mm\x1b[0m\x1b[33mp\x1b[0m\x1b[33ml\x1b[0m\x1b[33me\x1b[0m\x1b[33m2\x1b[0m\x1b[33m{\x1b[0m\x1b[32mInner\x1b[0m=A: test \x1b[32mOther\x1b[0m=\x1b[36m42\x1b[0m\x1b[33m}\x1b[0m\n\n",
	)

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testNoColor(t *testing.T, o *Options) {
	w := &MockWriter{}
	o.NoColor = true
	logger := slog.New(NewHandler(w, o))
	o.NoColor = false

	logger.Info("msg",
		slog.Any("i", 1),
		slog.Any("f", 2.2),
		slog.Any("s", "someString"),
		slog.Any("m", map[int]string{3: "three", 4: "four"}),
	)

	expected := []byte("[]  INFO  msg f=2.2 i=1 m=2 map[int]string{3=three 4=four} s=someString\n\n")

	if !bytes.Equal(w.WrittenData, expected) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testInfinite(t *testing.T, o *Options) {
	w := &MockWriter{}
	logger := slog.New(NewHandler(w, o))

	type Infinite struct {
		I *Infinite
	}

	v1 := Infinite{}
	v2 := Infinite{}
	v3 := Infinite{}

	v1.I = &v2
	v2.I = &v3
	v3.I = &v1

	logger.Info("msg",
		slog.Any("i", v1),
	)

	expected := fmt.Sprintf(
		"\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mi=\x1b[0m\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33mI\x1b[0m\x1b[33mn\x1b[0m\x1b[33mf\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mi\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33m{\x1b[0m\x1b[32mI\x1b[0m=\x1b[31m*\x1b[0m\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33mI\x1b[0m\x1b[33mn\x1b[0m\x1b[33mf\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mi\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33m{\x1b[0m\x1b[32mI\x1b[0m=\x1b[31m*\x1b[0m\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33mI\x1b[0m\x1b[33mn\x1b[0m\x1b[33mf\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mi\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33m{\x1b[0m\x1b[32mI\x1b[0m=\x1b[31m*\x1b[0m\x1b[33mh\x1b[0m\x1b[33mu\x1b[0m\x1b[33mm\x1b[0m\x1b[33ma\x1b[0m\x1b[33mn\x1b[0m\x1b[33ms\x1b[0m\x1b[33ml\x1b[0m\x1b[33mo\x1b[0m\x1b[33mg\x1b[0m\x1b[33m.\x1b[0m\x1b[33mI\x1b[0m\x1b[33mn\x1b[0m\x1b[33mf\x1b[0m\x1b[33mi\x1b[0m\x1b[33mn\x1b[0m\x1b[33mi\x1b[0m\x1b[33mt\x1b[0m\x1b[33me\x1b[0m\x1b[33m{\x1b[0m\x1b[32mI\x1b[0m=&{%p}\x1b[33m}\x1b[0m\x1b[33m}\x1b[0m\x1b[33m}\x1b[0m\x1b[33m}\x1b[0m\n\n",
		v2.I,
	)

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testSameSourceInfoColor(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: true,
	}

	o := &Options{
		HandlerOptions:      slogOpts,
		TimeFormat:          "[]",
		SameSourceInfoColor: true,
	}

	logger := slog.New(NewHandler(w, o))

	// Capture the line number right before the log call
	_, file, line, _ := runtime.Caller(0)
	logger.Info("msg",
		slog.Int("i", 1),
	)

	line++

	expected := fmt.Sprintf(
		"\x1b[2m[]\x1b[0m \x1b[37m%s:%d\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m msg \x1b[90mi=\x1b[0m\x1b[36m1\x1b[0m\n", file, line,
	)

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}
func TestOneLineFormat(t *testing.T) {
	testOneLineBasic(t)
	testOneLineWithAttributes(t)
	testOneLineWithQuoting(t)
	testOneLineWithGroups(t)
	testOneLineFallbackMessageNewline(t)
	testOneLineFallbackAttributeNewline(t)
	testOneLineWithColors(t)
	testOneLineNoColor(t)
	testOneLineWithSource(t)
	testOneLineWithJSONNewlines(t)
	testOneLineWithJSONSingleLine(t)
	testOneLineWithStructInline(t)
	testOneLineWithErrorChain(t)
	testOneLineWithErrorsJoin(t)
	testOneLineWithSliceInline(t)
	testOneLineWithMapInline(t)
	testOneLineWithMultilineFallbackUsesEquals(t)
	testOneLineNoPadding(t)
}

func testOneLineBasic(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Info("test message")

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test message\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineWithAttributes(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Info("test message",
		slog.String("foo", "bar"),
		slog.Int("count", 42),
		slog.Bool("active", true),
	)

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test message \x1b[90mfoo=\x1b[0mbar \x1b[90mcount=\x1b[0m\x1b[36m42\x1b[0m \x1b[90mactive=\x1b[0m\x1b[32mtrue\x1b[0m\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineWithQuoting(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Info("test message",
		slog.String("text", "value with spaces"),
		slog.String("equals", "key=value"),
	)

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test message \x1b[90mtext=\x1b[0mvalue with spaces \x1b[90mequals=\x1b[0mkey=value\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineWithGroups(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h).WithGroup("request")

	logger.Info("test message",
		slog.String("method", "GET"),
		slog.Int("status", 200),
	)

	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test message \x1b[90mrequest.method=\x1b[0mGET \x1b[90mrequest.status=\x1b[0m\x1b[36m200\x1b[0m\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineFallbackMessageNewline(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions:  slogOpts,
		TimeFormat:      "[]",
		NewLineAfterLog: true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Info("test\nmessage", slog.String("foo", "bar"))

	// Message with newlines is shown inline with spacing
	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m  \x1b[90mfoo=\x1b[0mbar  test\nmessage\n\n\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineFallbackAttributeNewline(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions:  slogOpts,
		TimeFormat:      "[]",
		NewLineAfterLog: true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Info("test message", slog.String("foo", "bar\nbaz"))

	// Attribute with newlines is shown inline with spacing
	expected := "\x1b[2m[]\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test message \x1b[90mfoo\x1b[0m=bar\nbaz\n\n\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineWithColors(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelWarn,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Warn("test message",
		slog.Int("count", 42),
		slog.Duration("duration", 5*time.Second),
	)

	expected := "\x1b[2m[]\x1b[0m \x1b[43m\x1b[30m WARN \x1b[0m test message \x1b[90mcount=\x1b[0m\x1b[36m42\x1b[0m \x1b[90mduration=\x1b[0m\x1b[37m5s\x1b[0m\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineNoColor(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
		NoColor:        true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	logger.Info("test message",
		slog.String("foo", "bar"),
		slog.Int("count", 42),
	)

	expected := "[]  INFO  test message foo=bar count=42\n"

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineWithSource(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	_, file, line, _ := runtime.Caller(0)
	logger.Info("test message", slog.String("foo", "bar"))
	line++

	expected := fmt.Sprintf("\x1b[2m[]\x1b[0m \x1b[37m%s:%d\x1b[0m \x1b[42m\x1b[30m INFO \x1b[0m test message \x1b[90mfoo=\x1b[0mbar\n", file, line)

	if !bytes.Equal(w.WrittenData, []byte(expected)) {
		t.Errorf("\nExpected:\n%s\nGot:\n%s\nExpected:\n%[1]q\nGot:\n%[2]q", expected, w.WrittenData)
	}
}

func testOneLineWithJSONNewlines(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions:  slogOpts,
		TimeFormat:      "[]",
		NewLineAfterLog: true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	// JSON string with newlines - should fall back to multi-line format
	jsonStr := `{
  "delivery_address": {
    "city": "Minneapolis",
    "country_code": "JP"
  },
  "items": [
    {
      "menu_item_uuid": "019b0d13-b157-70c3-b959-d7a488d20f33",
      "quantity": 3
    }
  ]
}`

	logger.Info("Create quote", slog.Any("order", jsonStr))

	// JSON with newlines is formatted with character-by-character coloring
	result := string(w.WrittenData)

	// Verify it uses one-line format (should have INFO badge)
	if !strings.Contains(result, "INFO") {
		t.Errorf("Expected one-line format with INFO badge, got:\n%s\n%q", result, result)
	}

	// Verify the JSON attribute name is present (colored as logfmt key)
	if !strings.Contains(result, "order") {
		t.Errorf("Expected 'order' attribute in output, got:\n%s\n%q", result, result)
	}

	// Verify JSON markers are present (character-by-character colored content)
	if !strings.Contains(result, "J") || !strings.Contains(result, "{") {
		t.Errorf("Expected JSON markers in output, got:\n%s\n%q", result, result)
	}
}

func testOneLineWithJSONSingleLine(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	// JSON string without newlines - should stay in one-line format
	jsonStr := `{"user":"john","id":123}`

	logger.Info("User data", slog.Any("data", jsonStr))

	result := string(w.WrittenData)

	// Should be in one-line format (should have INFO badge)
	if !strings.Contains(result, "INFO") {
		t.Errorf("Expected one-line format with INFO badge, got:\n%s\n%q", result, result)
	}

	// Verify the JSON attribute and markers are present (character-by-character colored)
	if !strings.Contains(result, "data") || !strings.Contains(result, "J") || !strings.Contains(result, "{") {
		t.Errorf("Expected JSON data in output, got:\n%s\n%q", result, result)
	}
}

func testOneLineWithStructInline(t *testing.T) {
	w := &MockWriter{}

	type Address struct {
		City    string
		Country string
	}

	type Person struct {
		Name    string
		Age     int
		Address Address
	}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	person := Person{
		Name: "John",
		Age:  30,
		Address: Address{
			City:    "NYC",
			Country: "USA",
		},
	}

	logger.Info("test", slog.Any("person", person))

	result := stripAnsi(string(w.WrittenData))

	// Should have struct type and all field names (package-qualified)
	// Structs are now multiline with : separator instead of inline with = separator
	if !strings.Contains(result, "humanslog.Person") {
		t.Errorf("Expected struct type 'humanslog.Person', got: %s", result)
	}
	if !strings.Contains(result, "Name") || !strings.Contains(result, "John") {
		t.Errorf("Expected field 'Name' with value 'John', got: %s", result)
	}
	if !strings.Contains(result, "Age") || !strings.Contains(result, "30") {
		t.Errorf("Expected field 'Age' with value '30', got: %s", result)
	}
	if !strings.Contains(result, "Address") {
		t.Errorf("Expected nested struct 'Address', got: %s", result)
	}
	if !strings.Contains(result, "City") || !strings.Contains(result, "NYC") {
		t.Errorf("Expected nested field 'City' with value 'NYC', got: %s", result)
	}
	// Should have INFO badge
	if !strings.Contains(result, "INFO") {
		t.Errorf("Expected INFO badge, got: %s", result)
	}
}

func testOneLineWithErrorChain(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	baseErr := errors.New("base error")
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)

	logger.Info("test", slog.Any("err", wrappedErr))

	result := string(w.WrittenData)

	// Should have both errors joined with ": "
	if !strings.Contains(result, "wrapped") {
		t.Errorf("Expected 'wrapped' in error, got: %s", result)
	}
	if !strings.Contains(result, "base error") {
		t.Errorf("Expected 'base error' in error, got: %s", result)
	}
	// Should be inline (no actual newlines in the error)
	lines := strings.Split(result, "\n")
	if len(lines) > 2 { // 1 line + trailing newline
		t.Errorf("Expected error to be inline, got multiple lines: %v", lines)
	}
}

func testOneLineWithErrorsJoin(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	err1 := errors.New("error one")
	err2 := errors.New("error two")
	joinedErr := errors.Join(err1, err2)

	logger.Info("test", slog.Any("err", joinedErr))

	result := string(w.WrittenData)

	// Should have both errors
	if !strings.Contains(result, "error one") {
		t.Errorf("Expected 'error one', got: %s", result)
	}
	if !strings.Contains(result, "error two") {
		t.Errorf("Expected 'error two', got: %s", result)
	}
	// Should be inline
	lines := strings.Split(result, "\n")
	if len(lines) > 2 {
		t.Errorf("Expected error to be inline, got multiple lines: %v", lines)
	}
}

func testOneLineWithSliceInline(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	items := []int{1, 2, 3, 4, 5}

	logger.Info("test", slog.Any("items", items))

	result := stripAnsi(string(w.WrittenData))

	// Should have slice type and count
	if !strings.Contains(result, "[]int{") {
		t.Errorf("Expected slice type '[]int{', got: %s", result)
	}
	// Should have all items
	for i := 1; i <= 5; i++ {
		if !strings.Contains(result, fmt.Sprintf("%d", i)) {
			t.Errorf("Expected item '%d', got: %s", i, result)
		}
	}
}

func testOneLineWithMapInline(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	data := map[string]int{
		"foo": 1,
		"bar": 2,
	}

	logger.Info("test", slog.Any("data", data))

	result := stripAnsi(string(w.WrittenData))

	// Should have map type
	if !strings.Contains(result, "map[string]int{") {
		t.Errorf("Expected map type 'map[string]int{', got: %s", result)
	}
	// Should have all entries
	if !strings.Contains(result, "foo=1") {
		t.Errorf("Expected 'foo=1', got: %s", result)
	}
	if !strings.Contains(result, "bar=2") {
		t.Errorf("Expected 'bar=2', got: %s", result)
	}
}

func testOneLineWithMultilineFallbackUsesEquals(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions: slogOpts,
		TimeFormat:     "[]",
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	// String with newlines triggers fallback, but other attrs should use =
	jsonStr := "{\n  \"key\": \"value\"\n}"

	logger.Info("test",
		slog.Any("json", jsonStr),
		slog.Int("count", 42),
	)

	result := stripAnsi(string(w.WrittenData))

	// Should use = separator (not :) even in fallback mode
	if !strings.Contains(result, "count=42") {
		t.Errorf("Expected 'count=42' with = separator, got: %s", result)
	}
	// Should NOT have "count: 42" with colon
	if strings.Contains(result, "count: 42") {
		t.Errorf("Should not use colon separator in OneLineFormat mode, got: %s", result)
	}
}

func testOneLineNoPadding(t *testing.T) {
	w := &MockWriter{}

	slogOpts := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	}

	opts := &Options{
		HandlerOptions:  slogOpts,
		TimeFormat:      "[]",
		NewLineAfterLog: true,
	}

	h := NewHandler(w, opts)
	logger := slog.New(h)

	// String with newlines triggers fallback
	jsonStr := "{\n  \"test\": true\n}"

	logger.Info("test",
		slog.Any("longkeyname", jsonStr),
		slog.String("x", "short"),
	)

	result := string(w.WrittenData)

	// Should NOT have padding spaces before = (like "x       =" in normal mode)
	// Should be "x=" directly
	if strings.Contains(result, "x       =") || strings.Contains(result, "x      =") {
		t.Errorf("Should not have padding before = in OneLineFormat mode, got: %s", result)
	}
}

// Helper to strip ANSI color codes for testing
func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}
