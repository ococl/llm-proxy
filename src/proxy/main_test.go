package proxy

import (
	"os"
	"testing"

	"llm-proxy/logging"
)

func TestMain(m *testing.M) {
	logging.SetTestMode(true)
	logging.InitTestLoggers()
	os.Exit(m.Run())
}
