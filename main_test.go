/*
This is just for local test with Spanner Emulator
Note: Before running this test, run spanner emulator and create an instance as "test-instance"
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/go-chi/chi/v5"
	gonanoid "github.com/matoous/go-nanoid"
	"github.com/shin5ok/egg6-architecting/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	fakeDbString = func() string {
		base := os.Getenv("SPANNER_STRING")
		parts := strings.Split(base, "/databases/")
		if len(parts) < 2 {
			// Fallback or error if SPANNER_STRING is not in the expected format
			// For now, let's assume it's just a base path that needs /databases/dbname
			if !strings.Contains(base, "/databases/") {
				if base == "" {
					base = "projects/your-project-id/instances/test-instance" // Default dummy
				}
				return base + "/databases/testdb" + genStr()
			}
			// If it contains /databases/ but split somehow failed to give 2 parts (unlikely for valid strings)
			log.Printf("Warning: SPANNER_STRING format is unexpected: %s", base)
			return base // return as is, hoping it's a full path or will fail clearly
		}
		// Correctly form the new database name
		return parts[0] + "/databases/testdb" + genStr()
	}()
	fakeServing  Serving

	itemTestID = "d169f397-ba3f-413b-bc3c-a465576ef06e"
	userTestID string
)

func genStr() string {
	var src = "abcdefghijklmnopqrstuvwxyz09123456789"
	id, err := gonanoid.Generate(src, 6)
	if err != nil {
		panic(err)
	}
	return string(id) + time.Now().Format("2006-01-02")
}

func init() {
	/*
	 for local emulator:
	 export SPANNER_STRING=projects/your-project-id/instances/test-instance/databases/game-dummy
	*/
	log.Println("Creating " + fakeDbString)

	if match, _ := regexp.MatchString("^projects/your-project-id/", fakeDbString); match {
		os.Setenv("SPANNER_EMULATOR_HOST", "localhost:9010")
	}
	ctx := context.Background()

	client, err := spanner.NewClient(ctx, fakeDbString)
	if err != nil {
		log.Printf("Warning: Failed to create Spanner client for fakeDbString '%s': %v. DB-dependent tests may fail.", fakeDbString, err)
		// Allow tests to continue, fakeServing.Client will be nil or its zero value
	} else {
		fakeServing = Serving{
			Client: dbClient{sc: client},
		}

		schemaFiles, _ := filepath.Glob("schemas/*_ddl.sql")
		if err := testutil.InitData(ctx, fakeDbString, schemaFiles); err != nil {
			log.Printf("Warning: Failed to init data for fakeDbString '%s': %v. DB-dependent tests may fail.", fakeDbString, err)
		}
	}
}

func Test_run(t *testing.T) {

	req, err := http.NewRequest("GET", "/", nil)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()
	localServing := Serving{} // Use local Serving instance
	handler := http.HandlerFunc(localServing.pingPong)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}

	// Check the response body
	expected := "Pong\n"
	assert.Equal(t, expected, rr.Body.String(), "handler returned unexpected body")

}

func TestHaikuHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/haiku", nil)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()
	localServing := Serving{} // Use local Serving instance
	handler := http.HandlerFunc(localServing.haikuHandler)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code, fmt.Sprintf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK))

	// Check the response body
	expectedHaiku := "Old silent pond...\nA frog jumps into the pond,\nsplash! Silence again."
	assert.Equal(t, expectedHaiku, rr.Body.String(), "handler returned unexpected body")
}

func TestTankaHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/tanka", nil)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()
	localServing := Serving{} // Use local Serving instance
	handler := http.HandlerFunc(localServing.tankaHandler)
	handler.ServeHTTP(rr, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, rr.Code, fmt.Sprintf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK))

	// Check the Content-Type header
	expectedContentType := "application/json; charset=utf-8"
	assert.Equal(t, expectedContentType, rr.Header().Get("Content-Type"), fmt.Sprintf("handler returned wrong content type: got %v want %v", rr.Header().Get("Content-Type"), expectedContentType))

	// Unmarshal the response body
	var body map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Nil(t, err, "Error unmarshalling response body")

	// Check if "tanka" key exists
	tankaValue, ok := body["tanka"]
	assert.True(t, ok, "Response body does not contain 'tanka' key")

	// Check if "tanka" value is non-empty
	assert.NotEmpty(t, tankaValue, "'tanka' value is empty")

	// Optionally, verify the tanka is one of the expected ones
	// tankaList is from main.go
	defaultTanka := "ふるさとの訛なつかし停車場の人ごみの中にそを聴きにゆく"
	expectedTankas := append(tankaList, defaultTanka)

	found := false
	for _, expected := range expectedTankas {
		if tankaValue == expected {
			found = true
			break
		}
	}
	assert.True(t, found, fmt.Sprintf("Returned tanka '%s' is not one of the expected tankas", tankaValue))
}

func Test_createUser(t *testing.T) {

	path := "test-user"
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_name", path)

	r := &http.Request{}
	req, err := http.NewRequestWithContext(r.Context(), "POST", "/api/user/"+path, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.createUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}
	var u User
	json.Unmarshal(rr.Body.Bytes(), &u)
	userTestID = u.Id

}

// This test depends on Test_createUser
func Test_addItemUser(t *testing.T) {

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_id", userTestID)
	ctx.URLParams.Add("item_id", itemTestID)

	r := &http.Request{}
	uriPath := fmt.Sprintf("/api/user_id/%s/%s", userTestID, itemTestID)
	req, err := http.NewRequestWithContext(r.Context(), "PUT", uriPath, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.addItemToUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}

}

func Test_getUserItems(t *testing.T) {

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_id", userTestID)

	r := &http.Request{}
	uriPath := fmt.Sprintf("/api/user_id/%s", userTestID)
	req, err := http.NewRequestWithContext(r.Context(), "GET", uriPath, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.addItemToUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}
}

func Test_cleaning(t *testing.T) {
	t.Cleanup(
		func() {
			ctx := context.Background()
			if err := testutil.DropData(ctx, fakeDbString); err != nil {
				t.Error(err)
			}
		},
	)
}
