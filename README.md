# 🧻 devslog - Go [slog.Handler](https://pkg.go.dev/log/slog#Handler) for development

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/roblaszczak/devslog/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/roblaszczak/devslog)](https://goreportcard.com/report/github.com/roblaszczak/devslog)
[![Go Reference](https://pkg.go.dev/badge/github.com/roblaszczak/devslog.svg)](https://pkg.go.dev/github.com/roblaszczak/devslog)

`devslog` is a zero dependency structured logging handler for Go's [`log/slog`](https://pkg.go.dev/log/slog) package with pretty and colorful output for developers.

This is an updated version of [github.com/golang-cz/devslog](https://github.com/golang-cz/devslog) that keeps the colorful formatting and structure but writes most of the log output in a **single line** for better readability. Multiline strings are preserved for readability, and JSON values are automatically formatted inline with syntax highlighting.
I also adjusted color choices to be more suitable for single-line output and closer to my personal preferences.

## Features

- Single-line log format (except for multiline strings)
- Inline JSON formatting with syntax highlighting
- Colorful output with customizable colors
- Zero dependencies
- Stack trace support for errors
- Logfmt-like output

## Install

```
go get github.com/roblaszczak/devslog@latest
```

## Examples

### Logger without options

```go
logger := slog.New(devslog.NewHandler(os.Stdout, nil))

// optional: set global logger
slog.SetDefault(logger)
```

### Logger with custom options

```go
// new logger with options
opts := &devslog.Options{
	MaxSlicePrintSize: 4,
	SortKeys:          true,
	TimeFormat:        "[04:05]",
	NewLineAfterLog:   true,
	DebugColor:        devslog.Magenta,
	StringerFormatter: true,
}

logger := slog.New(devslog.NewHandler(os.Stdout, opts))

// optional: set global logger
slog.SetDefault(logger)
```

### Logger with default slog options

Handler accepts default [slog.HandlerOptions](https://pkg.go.dev/golang.org/x/exp/slog#HandlerOptions)

```go
// slog.HandlerOptions
slogOpts := &slog.HandlerOptions{
	AddSource:   true,
	Level:       slog.LevelDebug,
}

// new logger with options
opts := &devslog.Options{
	HandlerOptions:    slogOpts,
	MaxSlicePrintSize: 4,
	SortKeys:          true,
	NewLineAfterLog:   true,
	StringerFormatter: true,
}

logger := slog.New(devslog.NewHandler(os.Stdout, opts))

// optional: set global logger
slog.SetDefault(logger)
```

### Example usage

```go
slogOpts := &slog.HandlerOptions{
	AddSource: true,
	Level:     slog.LevelDebug,
}

var logger *slog.Logger
if production {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, slogOpts))
} else {
	opts := &devslog.Options{
		HandlerOptions:    slogOpts,
		MaxSlicePrintSize: 10,
		SortKeys:          true,
		NewLineAfterLog:   true,
		StringerFormatter: true,
	}

	logger = slog.New(devslog.NewHandler(os.Stdout, opts))
}

// optional: set global logger
slog.SetDefault(logger)
```

## Options

| Parameter           | Description                                                    | Default        | Value                |
| ------------------- | -------------------------------------------------------------- | -------------- | -------------------- |
| MaxSlicePrintSize   | Specifies the maximum number of elements to print for a slice. | 50             | uint                 |
| SortKeys            | Determines if attributes should be sorted by keys.             | false          | bool                 |
| TimeFormat          | Time format for timestamp.                                     | "[15:04:05]"   | string               |
| NewLineAfterLog     | Add blank line after each log                                  | false          | bool                 |
| StringIndentation   | Indent \n in strings                                           | false          | bool                 |
| DebugColor          | Color for Debug level                                          | devslog.Blue   | devslog.Color (uint) |
| InfoColor           | Color for Info level                                           | devslog.Green  | devslog.Color (uint) |
| WarnColor           | Color for Warn level                                           | devslog.Yellow | devslog.Color (uint) |
| ErrorColor          | Color for Error level                                          | devslog.Red    | devslog.Color (uint) |
| MaxErrorStackTrace  | Max stack trace frames for errors                              | 0              | uint                 |
| StringerFormatter   | Use Stringer interface for formatting                          | false          | bool                 |
| NoColor             | Disable coloring                                               | false          | bool                 |
| SameSourceInfoColor | Keep same color for whole source info                          | false          | bool                 |

## Credits

This project is based on [github.com/golang-cz/devslog](https://github.com/golang-cz/devslog) created by the golang-cz community. Special thanks to all the contributors of the original project for building an excellent foundation for structured logging in Go.

The original project provided the colorful output, configuration options, and overall architecture that made this single-line variant possible.
