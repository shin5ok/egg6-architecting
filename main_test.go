package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/spannertest"
	"github.com/stretchr/testify/assert"
)

var (
	client       *spanner.Client
	fakeDbString = "projects/your-project-id/instances/foo-instance/databases/bar"
	fakeServing  = Serving{
		Client: dbClient{sc: client},
	}
)

func init() {
	srv, _ := spannertest.NewServer("localhost:0")
	// assert.Nil(t, err)
	os.Setenv("SPANNER_EMULATOR_HOST", srv.Addr)
	os.Setenv("PORT", "12820")
	ctx := context.Background()

	client, _ = spanner.NewClient(ctx, fakeDbString)

}

func Test_run(t *testing.T) {

	req, err := http.NewRequest("GET", "/", nil)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.pingPong)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d", http.StatusOK, rr.Code)
	}

}
