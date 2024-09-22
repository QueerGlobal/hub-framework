package api

import "github.com/rs/zerolog"

type LogLevel int

const (
	// Define log levels as constants
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// zerologLevelMapping maps LogLevel to zerolog.Level
var zerologLevelMapping = map[LogLevel]zerolog.Level{
	DebugLevel: zerolog.DebugLevel,
	InfoLevel:  zerolog.InfoLevel,
	WarnLevel:  zerolog.WarnLevel,
	ErrorLevel: zerolog.ErrorLevel,
	FatalLevel: zerolog.FatalLevel,
	PanicLevel: zerolog.PanicLevel,
}

// ToZeroLogLevel converts LogLevel to zerolog.Level
func (l LogLevel) ToZeroLogLevel() zerolog.Level {
	if lvl, ok := zerologLevelMapping[l]; ok {
		return lvl
	}
	// Default to InfoLevel if unknown
	return zerolog.InfoLevel
}
