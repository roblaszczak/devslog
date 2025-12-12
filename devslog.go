package humanslog

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type developHandler struct {
	opts Options
	goas []groupOrAttrs
	mu   sync.Mutex
	out  io.Writer
}

type Options struct {
	// You can use standard slog.HandlerOptions, that would be used in production
	*slog.HandlerOptions

	// Max number of printed elements in slice.
	MaxSlicePrintSize uint

	// If the attributes should be sorted by keys
	SortKeys bool

	// Time format for timestamp, default format is "[15:04:05]"
	TimeFormat string

	// Add blank line after each log
	NewLineAfterLog bool

	// Indent \n in strings
	StringIndentation bool

	// Set color for Debug level, default: humanslog.Blue
	DebugColor Color

	// Set color for Info level, default: humanslog.Green
	InfoColor Color

	// Set color for Warn level, default: humanslog.Yellow
	WarnColor Color

	// Set color for Error level, default: humanslog.Red
	ErrorColor Color

	// Max stack trace frames when unwrapping errors
	MaxErrorStackTrace uint

	// Use method String() for formatting value
	StringerFormatter bool

	// Disable coloring
	NoColor bool

	// Keep same color for whole source info, helpful when you want to open the line of code from terminal, but the ANSI coloring codes are in link itself
	SameSourceInfoColor bool
}

type groupOrAttrs struct {
	group string
	attrs []slog.Attr
}

func NewHandler(out io.Writer, o *Options) *developHandler {
	h := &developHandler{out: out}
	if o != nil {
		h.opts = *o

		if o.HandlerOptions != nil {
			h.opts.HandlerOptions = o.HandlerOptions
			if o.Level == nil {
				h.opts.Level = slog.LevelInfo
			} else {
				h.opts.Level = o.Level
			}
		} else {
			h.opts.HandlerOptions = &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}
		}

		if o.MaxSlicePrintSize == 0 {
			h.opts.MaxSlicePrintSize = 50
		}

		if o.TimeFormat == "" {
			h.opts.TimeFormat = "[15:04:05]"
		}

		h.opts.DebugColor = ensureValidColor(o.DebugColor, Blue)
		h.opts.InfoColor = ensureValidColor(o.InfoColor, Green)
		h.opts.WarnColor = ensureValidColor(o.WarnColor, Yellow)
		h.opts.ErrorColor = ensureValidColor(o.ErrorColor, Red)

	} else {
		h.opts = Options{
			HandlerOptions:    &slog.HandlerOptions{Level: slog.LevelInfo},
			MaxSlicePrintSize: 50,
			SortKeys:          false,
			TimeFormat:        "[15:04:05]",
			DebugColor:        Blue,
			InfoColor:         Green,
			WarnColor:         Yellow,
			ErrorColor:        Red,
		}
	}

	return h
}

func ensureValidColor(c Color, defaultColor Color) Color {
	if c > 0 && int(c) < len(colors) {
		return c
	}

	return defaultColor
}

func (h *developHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.opts.Level.Level()
}

func (h *developHandler) WithGroup(s string) slog.Handler {
	if s == "" {
		return h
	}

	return h.withGroupOrAttrs(groupOrAttrs{group: s})
}

func (h *developHandler) WithAttrs(as []slog.Attr) slog.Handler {
	if len(as) == 0 {
		return h
	}

	return h.withGroupOrAttrs(groupOrAttrs{attrs: as})
}

func (h *developHandler) withGroupOrAttrs(goa groupOrAttrs) *developHandler {
	h2 := &developHandler{
		opts: h.opts,
		goas: make([]groupOrAttrs, len(h.goas)+1),
		out:  h.out,
	}

	copy(h2.goas, h.goas)
	h2.goas[len(h2.goas)-1] = goa

	return h2
}

func (h *developHandler) Handle(ctx context.Context, r slog.Record) error {
	b := make([]byte, 0, 1024)

	// Use hybrid format: inline fields on one line + multiline fields at end
	b = h.formatOneLine(b, &r)

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.out.Write(b)

	return err
}

// containsMultiline checks if the message or any attribute contains newlines
func (h *developHandler) containsMultiline(r slog.Record) bool {
	// Check message
	if strings.Contains(r.Message, "\n") {
		return true
	}

	// Check all attributes
	hasNewline := false
	r.Attrs(func(a slog.Attr) bool {
		a.Value = a.Value.Resolve()
		if h.attrContainsNewline(a) {
			hasNewline = true
			return false // stop iteration
		}
		return true
	})

	return hasNewline
}

// attrContainsNewline recursively checks if an attribute contains newlines
// Only checks string types - other types (errors, structs, etc.) should stay inline
func (h *developHandler) attrContainsNewline(a slog.Attr) bool {
	switch a.Value.Kind() {
	case slog.KindString:
		return strings.Contains(a.Value.String(), "\n")
	case slog.KindGroup:
		for _, ga := range a.Value.Group() {
			if h.attrContainsNewline(ga) {
				return true
			}
		}
	case slog.KindAny:
		av := a.Value.Any()
		// Only check for newlines in plain string types
		if s, ok := av.(string); ok {
			return strings.Contains(s, "\n")
		}
		// Don't check errors, structs, or other types - keep them inline
	}
	return false
}

// formatOneLine formats the log record in a hybrid format:
// - One line with all inline fields (no newlines)
// - Multiline fields appended at the end in readable format
func (h *developHandler) formatOneLine(b []byte, r *slog.Record) []byte {
	// Timestamp
	b = append(b, h.faintedText([]byte(r.Time.Format(h.opts.TimeFormat)))...)
	b = append(b, ' ')

	// Source info if enabled
	if h.opts.AddSource {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		s := &slog.Source{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		}

		if h.opts.ReplaceAttr != nil {
			attr := h.opts.ReplaceAttr([]string{}, slog.Any(slog.SourceKey, s))
			if attr.Key != "" {
				sourceStr := fmt.Sprintf("%s:%d", s.File, s.Line)
				b = append(b, h.colorString([]byte(sourceStr), fgWhite)...)
				b = append(b, ' ')
			}
		} else {
			sourceStr := fmt.Sprintf("%s:%d", s.File, s.Line)
			b = append(b, h.colorString([]byte(sourceStr), fgWhite)...)
			b = append(b, ' ')
		}
	}

	// Level
	var ls string
	if h.opts.ReplaceAttr != nil {
		a := h.opts.ReplaceAttr(nil, slog.Any(slog.LevelKey, r.Level))
		ls = a.Value.String()
		if a.Key != "level" {
			r.AddAttrs(a)
		}
	} else {
		ls = r.Level.String()
	}

	var c color
	lr := r.Level
	switch {
	case lr < 0:
		c = h.getColor(h.opts.DebugColor)
	case lr < 4:
		c = h.getColor(h.opts.InfoColor)
	case lr < 8:
		c = h.getColor(h.opts.WarnColor)
	default:
		c = h.getColor(h.opts.ErrorColor)
	}

	// Level with badge (same as normal mode)
	b = append(b, h.colorStringBackgorund([]byte(" "+ls+" "), fgBlack, c.bg)...)
	b = append(b, ' ')

	// Message (only if no newlines - otherwise add to multiline section)
	messageHasNewlines := strings.Contains(r.Message, "\n")
	if !messageHasNewlines {
		b = append(b, h.colorString([]byte(r.Message), c.fg)...)
	}

	// Collect attributes
	var as attributes
	r.Attrs(func(a slog.Attr) bool {
		a.Value = a.Value.Resolve()
		as = append(as, a)
		return true
	})

	// Add pre-existing groups/attrs
	goas := h.goas
	if r.NumAttrs() == 0 {
		for len(goas) > 0 && goas[len(goas)-1].group != "" {
			goas = goas[:len(goas)-1]
		}
	}

	for i := len(goas) - 1; i >= 0; i-- {
		if goas[i].group != "" {
			ng := slog.Attr{
				Key:   goas[i].group,
				Value: slog.GroupValue(as...),
			}
			as = attributes{ng}
		} else {
			as = append(as, goas[i].attrs...)
		}
	}

	if h.opts.SortKeys {
		sort.Sort(as)
	}

	// Separate inline and multiline attributes
	var inlineAttrs, multilineAttrs attributes
	for _, a := range as {
		if h.attrContainsNewline(a) || h.isJSON(a.Value.String()) {
			multilineAttrs = append(multilineAttrs, a)
		} else {
			inlineAttrs = append(inlineAttrs, a)
		}
	}

	// Format inline attributes in logfmt on the same line
	b = h.formatLogfmtAttrs(b, inlineAttrs, []string{}, c.fg)

	// If message or any attributes have newlines, format them in multiline section
	if messageHasNewlines || len(multilineAttrs) > 0 {
		// Add message if it has newlines
		if messageHasNewlines {
			b = append(b, "  "...)
			b = append(b, h.colorString([]byte(r.Message), c.fg)...)
			b = append(b, '\n')
		}

		// Add multiline attributes
		if len(multilineAttrs) > 0 {
			vi := make(visited)
			b = h.colorize(b, multilineAttrs, 0, []string{}, vi)
		}
	}

	if h.opts.NewLineAfterLog {
		b = append(b, '\n')
	}
	b = append(b, '\n')

	return b
}

// formatLogfmtAttrs formats attributes in logfmt format
func (h *developHandler) formatLogfmtAttrs(b []byte, as attributes, group []string, levelColor foregroundColor) []byte {
	for _, a := range as {
		if h.opts.ReplaceAttr != nil {
			a = h.opts.ReplaceAttr(group, a)
		}

		// Handle groups by flattening with dot notation
		if a.Value.Kind() == slog.KindGroup {
			newGroup := append(group, a.Key)
			b = h.formatLogfmtAttrs(b, a.Value.Group(), newGroup, levelColor)
			continue
		}

		b = append(b, ' ')

		// Key (with group prefix if in a group)
		key := a.Key
		if len(group) > 0 {
			key = strings.Join(append(group, a.Key), ".")
		}
		// Color the "key=" together
		b = append(b, h.colorString([]byte(key+"="), fgGray)...)

		// Format value with detailed inline representation
		val := h.formatValueInline(a)
		b = append(b, val...)
	}

	return b
}

// formatLogfmtValue formats a value for logfmt, quoting if necessary
func (h *developHandler) formatLogfmtValue(val []byte, color foregroundColor) []byte {
	if color != nil {
		return h.colorString(val, color)
	}
	return val
}

func (h *developHandler) formatSourceInfo(b []byte, r *slog.Record) []byte {
	if h.opts.AddSource {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		s := &slog.Source{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		}

		if h.opts.ReplaceAttr != nil {
			attr := h.opts.ReplaceAttr([]string{}, slog.Any(slog.SourceKey, s))
			if attr.Key == "" {
				b = append(b, '\n')
				return b
			}
		}

		b = append(b, h.colorStringFainted([]byte("@@@"), fgYellow)...)
		b = append(b, ' ')

		if h.opts.SameSourceInfoColor {
			b = append(b, h.underlineText(h.colorStringFainted(append(append([]byte(s.File), ':'), []byte(strconv.Itoa(s.Line))...), fgWhite))...)
		} else {
			b = append(b, h.underlineText(h.colorStringFainted([]byte(s.File), fgWhite))...)
			b = append(b, h.faintedText([]byte(":"))...)
			b = append(b, h.colorStringFainted([]byte(strconv.Itoa(s.Line)), fgRed)...)
		}

		b = append(b, '\n')
	}

	return b
}

func (h *developHandler) levelMessage(b []byte, r *slog.Record) []byte {
	var ls string
	if h.opts.ReplaceAttr != nil {
		a := h.opts.ReplaceAttr(nil, slog.Any(slog.LevelKey, r.Level))
		ls = a.Value.String()
		if a.Key != "level" {
			r.AddAttrs(a)
		}
	} else {
		ls = r.Level.String()
	}

	var c color
	lr := r.Level
	switch {
	case lr < 0:
		c = h.getColor(h.opts.DebugColor)
	case lr < 4:
		c = h.getColor(h.opts.InfoColor)
	case lr < 8:
		c = h.getColor(h.opts.WarnColor)
	default:
		c = h.getColor(h.opts.ErrorColor)
	}

	b = append(b, h.colorStringBackgorund([]byte(" "+ls+" "), fgBlack, c.bg)...)
	b = append(b, ' ')
	b = append(b, h.colorString([]byte(r.Message), c.fg)...)
	b = append(b, '\n')

	return b
}

type visitKey struct {
	ptr uintptr
	typ reflect.Type
}

type visited map[visitKey]struct{}

func (h *developHandler) colorize(b []byte, as attributes, l int, group []string, vi visited) []byte {
	if h.opts.SortKeys {
		sort.Sort(as)
	}

	paddingNoColor := h.padding(as, group, nil, h.colorString)
	for _, a := range as {
		if h.opts.ReplaceAttr != nil {
			a = h.opts.ReplaceAttr(group, a)
		}

		key := h.colorString([]byte(a.Key), fgGray)
		val := []byte(a.Value.String())
		valOld := val
		vs := val
		mark := []byte{}

		switch a.Value.Kind() {
		case slog.KindFloat64, slog.KindInt64, slog.KindUint64:
			mark = h.colorString([]byte("#"), fgCyan)
			val = h.colorString(val, fgCyan)
		case slog.KindBool:
			c := fgRed
			if a.Value.Bool() {
				c = fgGreen
			}

			mark = h.colorString([]byte("#"), c)
			val = h.colorString(val, c)
		case slog.KindString:
			if len(val) == 0 {
				val = h.colorStringFainted([]byte("empty"), fgWhite)
			} else if h.isJSON(string(val)) {
				// Format as colorized JSON
				mark = h.colorString([]byte("J"), fgWhite)
				val = h.formatJSONMultiline(string(val), l)
			} else if h.isURL(val) {
				mark = h.colorString([]byte("*"), fgCyan)
				val = h.underlineText(h.colorString(val, fgCyan))
			} else {
				if h.opts.StringIndentation {
					count := l*2 + (4 + (paddingNoColor))
					val = []byte(strings.ReplaceAll(string(val), "\n", "\n"+strings.Repeat(" ", count)))
				}
			}
		case slog.KindTime, slog.KindDuration:
			mark = h.colorString([]byte("@"), fgWhite)
			val = h.colorString(val, fgWhite)
		case slog.KindAny:
			av := a.Value.Any()
			if err, ok := av.(error); ok {
				mark = h.colorString([]byte("E"), fgRed)
				// Always use inline format for errors
				val = h.formatError(err)
				break
			}

			if t, ok := av.(*time.Time); ok {
				mark = h.colorString([]byte("@"), fgWhite)
				val = h.colorString([]byte(t.String()), fgWhite)
				break
			}

			if d, ok := av.(*time.Duration); ok {
				mark = h.colorString([]byte("@"), fgWhite)
				val = h.colorString([]byte(d.String()), fgWhite)
				break
			}

			if textMarshaller, ok := av.(encoding.TextMarshaler); ok {
				val = atb(textMarshaller)
				break
			}

			if h.opts.StringerFormatter {
				if stringer, ok := av.(fmt.Stringer); ok {
					val = []byte(stringer.String())
					break
				}
			}

			avt := reflect.TypeOf(av)
			avv := reflect.ValueOf(av)
			if avt == nil {
				mark = h.colorString([]byte("!"), fgRed)
				val = h.nilString()
				break
			}

			ut, uv, ptrs := h.reducePointerTypeValue(avt, avv)
			val = bytes.Repeat(h.colorString([]byte("*"), fgRed), ptrs)

			switch ut.Kind() {
			case reflect.Array:
				mark = h.colorString([]byte("A"), fgGreen)
				val = h.formatSlice(avt, avv, vi)
			case reflect.Slice:
				mark = h.colorString([]byte("S"), fgGreen)
				val = h.formatSlice(avt, avv, vi)
			case reflect.Map:
				mark = h.colorString([]byte("M"), fgGreen)
				val = h.formatMap(avt, avv, vi)
			case reflect.Struct:
				mark = h.colorString([]byte("S"), fgYellow)
				val = h.formatStruct(avt, avv, vi)
			case reflect.Float32, reflect.Float64:
				mark = h.colorString([]byte("#"), fgCyan)
				vs = atb(uv.Float())
				val = append(val, h.colorString(vs, fgCyan)...)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				mark = h.colorString([]byte("#"), fgCyan)
				vs = atb(uv.Int())
				val = append(val, h.colorString(vs, fgCyan)...)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				mark = h.colorString([]byte("#"), fgCyan)
				vs = atb(uv.Uint())
				val = append(val, h.colorString(vs, fgCyan)...)
			case reflect.Bool:
				c := fgRed
				if uv.Bool() {
					c = fgGreen
				}

				mark = h.colorString([]byte("#"), c)
				vs = atb(uv.Bool())
				val = append(val, h.colorString(vs, c)...)
			case reflect.String:
				s := uv.String()
				if len(s) == 0 {
					val = h.colorStringFainted([]byte("empty"), fgWhite)
				} else if h.isURL([]byte(s)) {
					val = h.underlineText(h.colorString(val, fgCyan))
				} else {
					val = []byte(uv.String())
				}
			default:
				mark = h.colorString([]byte("!"), fgRed)
				val = h.colorString(atb("Unknown type"), fgRed)
			}
		case slog.KindGroup:
			mark = h.colorString([]byte("G"), fgGreen)
			var ga attributes
			ga = a.Value.Group()
			group = append(group, a.Key)

			val = []byte("\n")
			val = append(val, h.colorize(nil, ga, l+1, group, vi)...)
		}

		b = append(b, bytes.Repeat([]byte(" "), l*2)...)
		b = append(b, mark...)
		b = append(b, ' ')
		b = append(b, key...)
		// Only add padding for alignment when not in OneLineFormat mode

		b = append(b, []byte(h.separator())...)
		b = append(b, val...)

		stringer := reflect.ValueOf(a.Value).MethodByName("String")
		if stringer.IsValid() && !bytes.Equal(valOld, vs) {
			s := []byte(` "`)
			s = append(s, []byte(a.Value.String())...)
			s = append(s, '"')
			b = append(b, h.colorStringFainted(s, fgWhite)...)
		}

		if a.Value.Kind() != slog.KindGroup {
			b = append(b, '\n')
		}
	}

	return b
}

func (h *developHandler) separator() string {
	return "="
}

func (h *developHandler) padding(a attributes, g []string, color foregroundColor, colorFunction func(b []byte, fgColor foregroundColor) []byte) int {
	var padding int
	for _, attr := range a {
		if h.opts.ReplaceAttr != nil {
			attr = h.opts.ReplaceAttr(g, attr)
		}

		colorLength := len(attr.Key)
		if color != nil {
			colorLength = len(colorFunction([]byte(attr.Key), color))
		}

		if colorLength > padding {
			padding = colorLength
		}
	}

	return padding
}

func (h *developHandler) isURL(u []byte) bool {
	_, err := url.ParseRequestURI(string(u))
	return err == nil
}

func (h *developHandler) formatError(err error) []byte {
	var parts []string

	// Collect all error messages
	var collectErrors func(error)
	collectErrors = func(err error) {
		if err == nil {
			return
		}

		// Try to unwrap multiple errors (errors.Join)
		type unwrapMultiple interface {
			Unwrap() []error
		}
		if e, ok := err.(unwrapMultiple); ok {
			errs := e.Unwrap()
			for _, inner := range errs {
				collectErrors(inner)
			}
			return
		}

		// Try to unwrap single error
		ue := errors.Unwrap(err)
		if ue != nil {
			errMsg := err.Error()
			errMsg, _ = strings.CutSuffix(errMsg, ue.Error())
			errMsg, _ = strings.CutSuffix(errMsg, ": ")
			if errMsg != "" {
				parts = append(parts, errMsg)
			}
			collectErrors(ue)
		} else {
			// Leaf error
			parts = append(parts, err.Error())
		}
	}

	collectErrors(err)
	result := strings.Join(parts, ": ")
	return h.colorString([]byte(result), fgRed)
}

func (h *developHandler) formatSlice(st reflect.Type, sv reflect.Value, vi visited) []byte {
	ts := h.buildTypeString(st.String())
	_, sv, _ = h.reducePointerTypeValue(st, sv)

	b := h.colorString([]byte(strconv.Itoa(sv.Len())), fgCyan)
	b = append(b, ' ')
	b = append(b, ts...)
	b = append(b, h.colorString([]byte("{"), fgGreen)...)

	maxItems := min(int(h.opts.MaxSlicePrintSize), sv.Len())
	for i := 0; i < maxItems; i++ {
		if i > 0 {
			b = append(b, ' ')
		}
		v := sv.Index(i)
		b = append(b, h.elementType(v.Type(), v, vi)...)
	}
	if sv.Len() > maxItems {
		b = append(b, ' ')
		b = append(b, h.colorString([]byte("..."), fgCyan)...)
	}
	b = append(b, h.colorString([]byte("}"), fgGreen)...)
	return b
}

func (h *developHandler) formatMap(st reflect.Type, sv reflect.Value, vi visited) []byte {
	ts := h.buildTypeString(st.String())
	_, sv, _ = h.reducePointerTypeValue(st, sv)

	b := h.colorString([]byte(strconv.Itoa(sv.Len())), fgCyan)
	b = append(b, ' ')
	b = append(b, ts...)
	b = append(b, h.colorString([]byte("{"), fgGreen)...)

	sk := h.sortMapKeys(sv)
	for i, k := range sk {
		if i > 0 {
			b = append(b, ' ')
		}
		v := sv.MapIndex(k)
		v = h.reducePointerValue(v)
		k = h.reducePointerValue(k)

		b = append(b, h.colorString(atb(k.Interface()), fgGreen)...)
		b = append(b, '=')
		b = append(b, h.elementType(v.Type(), v, vi)...)
	}
	b = append(b, h.colorString([]byte("}"), fgGreen)...)
	return b
}

func (h *developHandler) formatStruct(st reflect.Type, sv reflect.Value, vi visited) []byte {
	b := h.buildTypeString(st.String())
	_, sv, _ = h.reducePointerTypeValue(st, sv)

	b = append(b, h.colorString([]byte("{"), fgYellow)...)
	first := true
	for i := 0; i < sv.NumField(); i++ {
		if !sv.Type().Field(i).IsExported() {
			continue
		}
		if !first {
			b = append(b, ' ')
		}
		first = false

		v := sv.Field(i)
		t := v.Type()

		b = append(b, h.colorString([]byte(sv.Type().Field(i).Name), fgGreen)...)
		b = append(b, '=')
		b = append(b, h.elementType(t, v, vi)...)
	}
	b = append(b, h.colorString([]byte("}"), fgYellow)...)
	return b
}

var marshalTextInterface = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

func (h *developHandler) elementType(t reflect.Type, v reflect.Value, vi visited) []byte {
	if t.Implements(marshalTextInterface) {
		return atb(v)
	}

	if h.opts.StringerFormatter {
		if stringer, ok := v.Interface().(fmt.Stringer); ok {
			return []byte(stringer.String())
		}
	}

	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		return h.formatSlice(t, v, vi)
	case reflect.Map:
		return h.formatMap(t, v, vi)
	case reflect.Struct:
		return h.formatStruct(t, v, vi)
	case reflect.Pointer:
		key := visitKey{
			ptr: v.Pointer(),
			typ: v.Type(),
		}
		if v.IsNil() {
			return h.nilString()
		} else if _, ok := vi[key]; ok {
			return atb(v)
		} else {
			vi[key] = struct{}{}
			return h.elementType(t, v.Elem(), vi)
		}
	case reflect.Float32, reflect.Float64:
		return h.colorString(atb(v.Float()), fgCyan)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return h.colorString(atb(v.Int()), fgCyan)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return h.colorString(atb(v.Uint()), fgCyan)
	case reflect.Bool:
		c := fgRed
		if v.Bool() {
			c = fgGreen
		}

		return h.colorString(atb(v.Bool()), c)
	case reflect.String:
		s := v.String()
		if len(s) == 0 {
			return h.colorStringFainted([]byte("empty"), fgWhite)
		}
		return atb(s)
	case reflect.Interface:
		if v.IsZero() {
			return h.nilString()
		}
		v = reflect.ValueOf(v.Interface())
		return h.elementType(v.Type(), v, vi)
	default:
		return atb(v)
	}
}

// Inline formatters for OneLineFormat mode

func (h *developHandler) formatValueInline(a slog.Attr) []byte {
	vi := make(visited)

	switch a.Value.Kind() {
	case slog.KindString:
		val := []byte(a.Value.String())
		if h.isJSON(string(val)) {
			// Format as colorized JSON inline
			jsonVal := h.formatJSONMultiline(string(val), 0)
			return h.formatLogfmtValue(jsonVal, nil)
		}
		if h.isURL(val) {
			return h.formatLogfmtValue(val, fgCyan)
		}
		return h.formatLogfmtValue(val, nil)
	case slog.KindFloat64, slog.KindInt64, slog.KindUint64:
		val := []byte(a.Value.String())
		return h.formatLogfmtValue(val, fgCyan)
	case slog.KindBool:
		c := fgRed
		if a.Value.Bool() {
			c = fgGreen
		}

		val := []byte(a.Value.String())
		return h.formatLogfmtValue(val, c)
	case slog.KindTime, slog.KindDuration:
		val := []byte(a.Value.String())
		return h.formatLogfmtValue(val, fgWhite)
	case slog.KindAny:
		av := a.Value.Any()

		// Error - use inline formatter
		if err, ok := av.(error); ok {
			return h.formatLogfmtValue(h.formatError(err), nil)
		}

		// Time types
		if t, ok := av.(*time.Time); ok {
			val := []byte(t.String())
			return h.formatLogfmtValue(val, fgWhite)
		}
		if d, ok := av.(*time.Duration); ok {
			val := []byte(d.String())
			return h.formatLogfmtValue(val, fgWhite)
		}
		if d, ok := av.([]uint8); ok && utf8.Valid(d) {
			av = string(d)
		}

		// Text marshaler
		if textMarshaller, ok := av.(encoding.TextMarshaler); ok {
			return h.formatLogfmtValue(atb(textMarshaller), nil)
		}

		// Stringer
		if h.opts.StringerFormatter {
			if stringer, ok := av.(fmt.Stringer); ok {
				return h.formatLogfmtValue([]byte(stringer.String()), nil)
			}
		}

		// Reflect-based types
		avt := reflect.TypeOf(av)
		avv := reflect.ValueOf(av)
		if avt == nil {
			return h.formatLogfmtValue(h.nilString(), nil)
		}

		ut, uv, ptrs := h.reducePointerTypeValue(avt, avv)
		prefix := bytes.Repeat(h.colorString([]byte("*"), fgRed), ptrs)

		switch ut.Kind() {
		case reflect.Array, reflect.Slice:
			val := h.formatSlice(avt, avv, vi)
			return h.formatLogfmtValue(append(prefix, val...), nil)
		case reflect.Map:
			val := h.formatMap(avt, avv, vi)
			return h.formatLogfmtValue(append(prefix, val...), nil)
		case reflect.Struct:
			val := h.formatStruct(avt, avv, vi)
			return h.formatLogfmtValue(append(prefix, val...), nil)
		case reflect.Float32, reflect.Float64:
			val := atb(uv.Float())
			return h.formatLogfmtValue(append(prefix, h.colorString(val, fgCyan)...), nil)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			val := atb(uv.Int())
			return h.formatLogfmtValue(append(prefix, h.colorString(val, fgCyan)...), nil)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			val := atb(uv.Uint())
			return h.formatLogfmtValue(append(prefix, h.colorString(val, fgCyan)...), nil)
		case reflect.Bool:
			c := fgRed
			if uv.Bool() {
				c = fgGreen
			}

			val := atb(uv.Bool())
			return h.formatLogfmtValue(append(prefix, h.colorString(val, c)...), nil)
		case reflect.String:
			s := uv.String()
			if len(s) == 0 {
				return h.formatLogfmtValue(append(prefix, h.colorStringFainted([]byte("empty"), fgWhite)...), nil)
			}
			if h.isURL([]byte(s)) {
				return h.formatLogfmtValue(append(prefix, []byte(s)...), fgCyan)
			}
			if h.isJSON(s) {
				// Format as colorized JSON inline
				jsonVal := h.formatJSONMultiline(s, 0)
				return h.formatLogfmtValue(jsonVal, nil)
			}

			return h.formatLogfmtValue(append(prefix, []byte(s)...), nil)
		default:
			return h.formatLogfmtValue([]byte(a.Value.String()), nil)
		}
	default:
		return h.formatLogfmtValue([]byte(a.Value.String()), nil)
	}
}

func (h *developHandler) buildTypeString(ts string) (b []byte) {
	t := []byte(ts)

	for len(t) > 0 {
		switch t[0] {
		case '*':
			b = append(b, h.colorString([]byte{t[0]}, fgRed)...)
		case '[', ']':
			b = append(b, h.colorString([]byte{t[0]}, fgGreen)...)
		default:
			b = append(b, h.colorString([]byte{t[0]}, fgYellow)...)
		}

		t = t[1:]
	}

	return b
}

func (h *developHandler) sortMapKeys(rv reflect.Value) []reflect.Value {
	ks := make([]reflect.Value, 0, rv.Len())
	ks = append(ks, rv.MapKeys()...)

	sort.Slice(ks, func(i, j int) bool {
		return fmt.Sprint(ks[i].Interface()) < fmt.Sprint(ks[j].Interface())
	})

	return ks
}

func (h *developHandler) reducePointerValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	return v
}

func (h *developHandler) reducePointerTypeValue(t reflect.Type, v reflect.Value) (reflect.Type, reflect.Value, int) {
	if t == nil {
		return t, v, 0
	}

	var ptr int
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
		if isNilValue(v) {
			return t, v, ptr
		}

		t = v.Type()

		ptr++
	}

	return t, v, ptr
}

// Any to []byte using fmt.Sprintf
func atb(a any) []byte {
	return fmt.Appendf(nil, "%v", a)
}

func isNilValue(v reflect.Value) bool {
	nilValue := reflect.ValueOf(nil)
	return v == nilValue
}

func (h *developHandler) nilString() []byte {
	return h.colorString([]byte("<nil>"), fgYellow)
}

// isJSON checks if a string value is valid JSON
func (h *developHandler) isJSON(val string) bool {
	// Quick check: must start with {
	trimmed := strings.TrimSpace(val)
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return false
	}

	// Try to validate as JSON
	var js json.RawMessage
	return json.Unmarshal([]byte(trimmed), &js) == nil
}

// formatJSONInline formats JSON string with colors in a compact single-line format
func (h *developHandler) formatJSONInline(jsonStr string) []byte {
	trimmed := strings.TrimSpace(jsonStr)

	// Compact the JSON first (remove extra whitespace)
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(trimmed)); err != nil {
		return []byte(jsonStr) // Return original if compacting fails
	}

	return h.colorizeJSONBytes(compact.Bytes(), false, 0)
}

// formatJSONMultiline formats JSON string with colors and indentation
func (h *developHandler) formatJSONMultiline(jsonStr string, baseIndent int) []byte {
	trimmed := strings.TrimSpace(jsonStr)

	// Pretty print the JSON
	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(trimmed), strings.Repeat(" ", baseIndent*2), "  "); err != nil {
		return []byte(jsonStr) // Return original if indenting fails
	}

	return h.colorizeJSONBytes(indented.Bytes(), true, baseIndent)
}

// colorizeJSONBytes adds colors to JSON bytes
func (h *developHandler) colorizeJSONBytes(data []byte, multiline bool, baseIndent int) []byte {
	var result []byte
	inString := false
	inKey := false
	escape := false

	for i := 0; i < len(data); i++ {
		ch := data[i]

		if escape {
			result = append(result, ch)
			escape = false
			continue
		}

		if ch == '\\' && inString {
			escape = true
			result = append(result, ch)
			continue
		}

		switch ch {
		case '"':
			if !inString {
				// Start of string - determine if it's a key or value
				inString = true
				// Look ahead to see if this is followed by a colon (making it a key)
				isKey := false
				// First, skip past the string content to find the closing quote
				j := i + 1
				escapeNext := false
				for j < len(data) {
					if escapeNext {
						escapeNext = false
						j++
						continue
					}
					if data[j] == '\\' {
						escapeNext = true
						j++
						continue
					}
					if data[j] == '"' {
						// Found closing quote, now look for colon after it
						j++
						break
					}
					j++
				}
				// Now skip whitespace and check for colon
				for j < len(data) {
					if data[j] == ':' {
						isKey = true
						break
					} else if data[j] != ' ' && data[j] != '\n' && data[j] != '\t' && data[j] != '\r' {
						// Hit something other than whitespace or colon
						break
					}
					j++
				}
				inKey = isKey
				if inKey {
					result = append(result, h.colorString([]byte{ch}, fgGray)...)
				} else {
					result = append(result, h.colorString([]byte{ch}, fgWhite)...)
				}
			} else {
				// End of string
				if inKey {
					result = append(result, h.colorString([]byte{ch}, fgGray)...)
				} else {
					result = append(result, h.colorString([]byte{ch}, fgWhite)...)
				}
				inString = false
				inKey = false
			}
		case '{', '}', '[', ']':
			result = append(result, h.colorString([]byte{ch}, fgCyan)...)
		case ':':
			result = append(result, h.colorString([]byte{ch}, fgWhite)...)
		case ',':
			result = append(result, h.colorString([]byte{ch}, fgWhite)...)
		case 't', 'f': // true/false
			if !inString {
				// Check if this is the start of true or false
				if i+3 < len(data) && string(data[i:i+4]) == "true" {
					result = append(result, h.colorString([]byte("true"), fgGreen)...)
					i += 3
				} else if i+4 < len(data) && string(data[i:i+5]) == "false" {
					result = append(result, h.colorString([]byte("false"), fgRed)...)
					i += 4
				} else {
					result = append(result, ch)
				}
			} else {
				if inKey {
					result = append(result, h.colorString([]byte{ch}, fgGray)...)
				} else {
					result = append(result, h.colorString([]byte{ch}, fgWhite)...)
				}
			}
		case 'n': // null
			if !inString && i+3 < len(data) && string(data[i:i+4]) == "null" {
				result = append(result, h.colorString([]byte("null"), fgBlack)...)
				i += 3
			} else if inString {
				if inKey {
					result = append(result, h.colorString([]byte{ch}, fgGray)...)
				} else {
					result = append(result, h.colorString([]byte{ch}, fgWhite)...)
				}
			} else {
				result = append(result, ch)
			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '.':
			if !inString {
				// Start of number - collect the whole number
				numStart := i
				for i < len(data) && (data[i] >= '0' && data[i] <= '9' || data[i] == '.' || data[i] == '-' || data[i] == 'e' || data[i] == 'E' || data[i] == '+') {
					i++
				}
				i-- // Back up one since the loop will increment
				result = append(result, h.colorString(data[numStart:i+1], fgCyan)...)
			} else {
				if inKey {
					result = append(result, h.colorString([]byte{ch}, fgGray)...)
				} else {
					result = append(result, h.colorString([]byte{ch}, fgWhite)...)
				}
			}
		default:
			if inString {
				if inKey {
					result = append(result, h.colorString([]byte{ch}, fgGray)...)
				} else {
					result = append(result, h.colorString([]byte{ch}, fgWhite)...)
				}
			} else {
				result = append(result, ch)
			}
		}
	}

	return result
}
