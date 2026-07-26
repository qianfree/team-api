package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseCaptureWriterCommitted(t *testing.T) {
	t.Run("fresh", func(t *testing.T) {
		capture := NewResponseCaptureWriter(httptest.NewRecorder())
		if capture.ResponseCommitted() {
			t.Fatal("fresh writer should not be committed")
		}
	})

	t.Run("header", func(t *testing.T) {
		capture := NewResponseCaptureWriter(httptest.NewRecorder())
		capture.WriteHeader(http.StatusAccepted)
		if !capture.ResponseCommitted() {
			t.Fatal("writer should be committed after WriteHeader")
		}
		if got := capture.StatusCode(); got != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", got, http.StatusAccepted)
		}
	})

	t.Run("body", func(t *testing.T) {
		capture := NewResponseCaptureWriter(httptest.NewRecorder())
		_, _ = capture.Write([]byte("partial"))
		if !capture.ResponseCommitted() {
			t.Fatal("writer should be committed after Write")
		}
		capture.WriteHeader(http.StatusInternalServerError)
		if got := capture.StatusCode(); got != http.StatusOK {
			t.Fatalf("status = %d, want implicit status %d", got, http.StatusOK)
		}
	})
}

func TestResponseCaptureWriterKeepsFirstStatus(t *testing.T) {
	capture := NewResponseCaptureWriter(httptest.NewRecorder())
	capture.WriteHeader(http.StatusAccepted)
	capture.WriteHeader(http.StatusInternalServerError)
	if got := capture.StatusCode(); got != http.StatusAccepted {
		t.Fatalf("status = %d, want first status %d", got, http.StatusAccepted)
	}
}
