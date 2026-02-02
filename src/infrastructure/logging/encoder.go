package logging

import (
	"bytes"
	"fmt"
	"regexp"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var reqIDPattern = regexp.MustCompile(`\[req_[a-zA-Z0-9]+\]`)

// maskingCore 脱敏核心
type maskingCore struct {
	zapcore.Core
	masker *SensitiveDataMasker
}

func (c *maskingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = c.masker.Mask(entry.Message)
	for i := range fields {
		if fields[i].Type == zapcore.StringType {
			fields[i].String = c.masker.Mask(fields[i].String)
		}
	}
	return c.Core.Write(entry, fields)
}

func (c *maskingCore) With(fields []zapcore.Field) zapcore.Core {
	return &maskingCore{Core: c.Core.With(fields), masker: c.masker}
}

func (c *maskingCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// SensitiveDataMasker 敏感数据脱敏器
type SensitiveDataMasker struct{}

func NewSensitiveDataMasker() *SensitiveDataMasker {
	return &SensitiveDataMasker{}
}

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(pk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
	regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(password["\s:=]+)([a-zA-Z0-9\-_!@#$%^&*()]{8,})`),
	regexp.MustCompile(`(?i)(token["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
	regexp.MustCompile(`(?i)(secret["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
}

func (m *SensitiveDataMasker) Mask(data string) string {
	result := data
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			if len(match) > 8 {
				return match[:4] + "****" + match[len(match)-4:]
			}
			return "****"
		})
	}
	return result
}

// markdownConsoleEncoder Markdown 格式控制台编码器
type markdownConsoleEncoder struct {
	zapcore.Encoder
	colored      bool
	consoleStyle string
}

func newMarkdownConsoleEncoder(cfg zapcore.EncoderConfig, colored bool, consoleStyle string) zapcore.Encoder {
	return &markdownConsoleEncoder{
		Encoder:      zapcore.NewConsoleEncoder(cfg),
		colored:      colored,
		consoleStyle: consoleStyle,
	}
}

func (enc *markdownConsoleEncoder) Clone() zapcore.Encoder {
	return &markdownConsoleEncoder{
		Encoder:      enc.Encoder.Clone(),
		colored:      enc.colored,
		consoleStyle: enc.consoleStyle,
	}
}

func (enc *markdownConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := bytes.NewBuffer(nil)

	timeStr := entry.Time.Format("15:04:05")
	if enc.colored {
		line.WriteString("\033[90m")
		line.WriteString(timeStr)
		line.WriteString("\033[0m")
	} else {
		line.WriteString(timeStr)
	}
	line.WriteString(" | ")

	levelStr := entry.Level.CapitalString()
	levelFormatted := fmt.Sprintf("%5s", levelStr)
	if enc.colored {
		switch entry.Level {
		case zapcore.DebugLevel:
			line.WriteString("\033[35m")
		case zapcore.InfoLevel:
			line.WriteString("\033[32m")
		case zapcore.WarnLevel:
			line.WriteString("\033[33m")
		case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
			line.WriteString("\033[31m")
		}
		line.WriteString(levelFormatted)
		line.WriteString("\033[0m")
	} else {
		line.WriteString(levelFormatted)
	}
	line.WriteString(" | ")

	var reqID, backendModel, backend, protocol string
	filteredFields := make([]zapcore.Field, 0, len(fields))
	for _, field := range fields {
		switch field.Key {
		case "req_id":
			if field.Type == zapcore.StringType {
				reqID = field.String
			}
		case "backend_model":
			if field.Type == zapcore.StringType {
				backendModel = field.String
			}
		case "backend":
			if field.Type == zapcore.StringType {
				backend = field.String
			}
		case "protocol":
			if field.Type == zapcore.StringType {
				protocol = field.String
			}
		case "logger":
		default:
			filteredFields = append(filteredFields, field)
		}
	}

	if reqID != "" {
		colorMgr := GetGlobalColorManager()
		reqColor := colorMgr.GetRequestColor(reqID)
		if enc.colored && reqColor != "" {
			line.WriteString(reqColor)
			line.WriteString(reqID)
			line.WriteString("\033[0m")
		} else {
			line.WriteString(reqID)
		}
	}
	line.WriteString(" | ")

	msg := entry.Message
	line.WriteString(msg)

	if backend != "" {
		colorMgr := GetGlobalColorManager()
		reqColor := colorMgr.GetRequestColor(reqID)
		if enc.colored && reqColor != "" {
			line.WriteString(" [backend=")
			line.WriteString(reqColor)
			line.WriteString(backend)
			line.WriteString("\033[0m")
			if backendModel != "" {
				line.WriteString(", model=")
				line.WriteString(reqColor)
				line.WriteString(backendModel)
				line.WriteString("\033[0m")
			}
			if protocol != "" {
				line.WriteString(", protocol=")
				line.WriteString(reqColor)
				line.WriteString(protocol)
				line.WriteString("\033[0m")
			}
			line.WriteString("]")
		} else {
			line.WriteString(" [backend=")
			line.WriteString(backend)
			if backendModel != "" {
				line.WriteString(", model=")
				line.WriteString(backendModel)
			}
			if protocol != "" {
				line.WriteString(", protocol=")
				line.WriteString(protocol)
			}
			line.WriteString("]")
		}
	}

	if len(filteredFields) > 0 {
		enc.encodeFields(line, filteredFields)
	}

	line.WriteString("\n")

	buf := buffer.NewPool().Get()
	buf.Write(line.Bytes())
	return buf, nil
}

func (enc *markdownConsoleEncoder) encodeFields(line *bytes.Buffer, fields []zapcore.Field) {
	if enc.consoleStyle == "compact" {
		line.WriteString(" [")
		firstField := true
		for _, field := range fields {
			if field.Key == "logger" {
				continue
			}
			if !firstField {
				line.WriteString(", ")
			}
			firstField = false
			enc.writeField(line, field)
		}
		line.WriteString("]")
	} else {
		for _, field := range fields {
			if field.Key == "logger" {
				continue
			}
			line.WriteString("\n  ")
			enc.writeField(line, field)
		}
	}
}

func (enc *markdownConsoleEncoder) writeField(line *bytes.Buffer, field zapcore.Field) {
	if enc.colored {
		line.WriteString("\033[90m")
		line.WriteString(field.Key)
		line.WriteString("\033[0m=\033[33m")
		line.WriteString(fieldValueString(field))
		line.WriteString("\033[0m")
	} else {
		line.WriteString(field.Key)
		line.WriteString("=")
		line.WriteString(fieldValueString(field))
	}
}

func fieldValueString(field zapcore.Field) string {
	switch field.Type {
	case zapcore.StringType:
		return field.String
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type,
		zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		return fmt.Sprintf("%d", field.Integer)
	case zapcore.BoolType:
		if field.Integer == 1 {
			return "true"
		}
		return "false"
	default:
		return field.String
	}
}
