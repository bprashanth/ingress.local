package http2

import (
	"net/http"
	"testing"
)

func TestTransport(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://ip.appspot.com/", nil)
	rt := &Transport{}
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("%v", err)
	}
}
