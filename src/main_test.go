package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	SetTestMode(true)
	os.Exit(m.Run())
}
