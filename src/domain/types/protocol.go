package types

// Protocol represents the API protocol of a backend.
type Protocol string

const (
	ProtocolOpenAI     Protocol = "openai"
	ProtocolAnthropic  Protocol = "anthropic"
)

func (p Protocol) String() string {
	return string(p)
}

func (p Protocol) IsValid() bool {
	return p == ProtocolOpenAI || p == ProtocolAnthropic
}
