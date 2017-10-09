package log

import "time"

type (
	Logger interface {
		WithFields(fields Fields) Logger
		Debug(fields Fields, format string, args ...interface{})
		Info(fields Fields, format string, args ...interface{})
		Warning(fields Fields, format string, args ...interface{})
		Error(fields Fields, format string, args ...interface{})
		Fatal(fields Fields, format string, args ...interface{})
		Sync() error
	}

	Fields map[string]interface{}
)

const (
	ConsoleTimeFormat = "2006-01-02 15:04:05.000"
	JSONTimeFormat    = "2006-01-02T15:04:05.000-0700"
)

func (f Fields) normalizeTimeValues() Fields {
	for key, val := range f {
		switch v := val.(type) {
		case time.Time:
			f[key] = v.Format(JSONTimeFormat)
		default:
			f[key] = v
		}
	}

	return f
}
