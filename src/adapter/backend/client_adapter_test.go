package backend

import (
	"net/http"
	"testing"
)

func TestBackendClientAdapter_Send_Success(t *testing.T) {
	mockClient := &http.Client{}
	httpClient := NewHTTPClient(mockClient)
	adapter := NewBackendClientAdapter(httpClient)

	if adapter == nil {
		t.Error("BackendClientAdapter should not be nil")
	}

	if adapter.GetHTTPClient() != mockClient {
		t.Error("GetHTTPClient should return the underlying HTTP client")
	}
}

func TestBackendClientAdapter_Send_WithAllOptions(t *testing.T) {
	mockClient := &http.Client{}
	httpClient := NewHTTPClient(mockClient)
	adapter := NewBackendClientAdapter(httpClient)

	if adapter == nil {
		t.Fatal("BackendClientAdapter should not be nil")
	}

	httpClientFromAdapter := adapter.GetHTTPClient()
	if httpClientFromAdapter == nil {
		t.Error("GetHTTPClient should not return nil")
	}
}

func TestHTTPClient_NewHTTPClient_NilClient(t *testing.T) {
	client := NewHTTPClient(nil)
	if client == nil {
		t.Error("NewHTTPClient should not return nil even with nil input")
	}
	if client.GetHTTPClient() == nil {
		t.Error("HTTP client should use http.DefaultClient when nil is passed")
	}
}

func TestHTTPClient_NewHTTPClient_WithClient(t *testing.T) {
	mockClient := &http.Client{}
	client := NewHTTPClient(mockClient)

	if client.GetHTTPClient() != mockClient {
		t.Error("NewHTTPClient should use the provided client")
	}
}
