package port

import "time"

const (
	FieldTraceID     = "trace_id"
	FieldBackend     = "backend"
	FieldModel       = "model"
	FieldErrorType   = "error_type"
	FieldErrorCode   = "error_code"
	FieldMessage     = "message"
	FieldDurationMS  = "duration_ms"
	FieldAttempt     = "attempt"
	FieldMaxRetries  = "max_retries"
	FieldStatusCode  = "status_code"
	FieldPriority    = "priority"
	FieldDelayMS     = "delay_ms"
	FieldNextAttempt = "next_attempt"
)

func TraceID(id string) Field {
	return String(FieldTraceID, id)
}

func Backend(name string) Field {
	return String(FieldBackend, name)
}

func Model(name string) Field {
	return String(FieldModel, name)
}

func DurationMS(d time.Duration) Field {
	return Int64(FieldDurationMS, d.Milliseconds())
}

func Attempt(n int) Field {
	return Int(FieldAttempt, n)
}

func MaxRetries(n int) Field {
	return Int(FieldMaxRetries, n)
}

func StatusCode(code int) Field {
	return Int(FieldStatusCode, code)
}

func Priority(p int) Field {
	return Int(FieldPriority, p)
}

func DelayMS(d time.Duration) Field {
	return Int64(FieldDelayMS, d.Milliseconds())
}
