package webpush

import (
	"context"
	"testing"
)

func TestPublicDialRejectsPrivateEndpoints(t *testing.T) {
	for _, address := range []string{"127.0.0.1:443", "10.0.0.1:443", "[::1]:443"} {
		if connection, err := publicDial(context.Background(), "tcp", address); err == nil {
			connection.Close()
			t.Fatalf("accepted private endpoint %s", address)
		}
	}
}
