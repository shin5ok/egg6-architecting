package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	chiprometheus "github.com/766b/chi-prometheus"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var appName = "myapp"

var spannerString = os.Getenv("SPANNER_STRING")
var servicePort = os.Getenv("PORT")

type Serving struct {
	Client GameUserOperation
}

type User struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

func main() {

	ctx := context.Background()

	client, err := newClient(ctx, spannerString)
	if err != nil {
		log.Fatal(err)
	}
	defer client.sc.Close()

	s := Serving{
		Client: client,
	}

	oplog := httplog.LogEntry(context.Background())
	/* jsonify logging */
	httpLogger := httplog.NewLogger(appName, httplog.Options{JSON: true, LevelFieldName: "severity", Concise: true})

	/* exporter for prometheus */
	m := chiprometheus.NewMiddleware(appName)

	r := chi.NewRouter()
	// r.Use(middleware.Throttle(8))
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(httplog.RequestLogger(httpLogger))
	r.Use(m)

	r.Handle("/metrics", promhttp.Handler())

	r.Get("/ping", s.pingPong)

	r.Route("/api", func(t chi.Router) {
		t.Get("/user_id/{user_id:[a-z0-9-.]+}", s.getUserItems)
		t.Post("/user/{user_name:[a-z0-9-.]+}", s.createUser)
		t.Put("/user_id/{user_id:[a-z0-9-.]+}/{item_id:[a-z0-9-.]+}", s.addItemToUser)
	})

	if err := http.ListenAndServe(":"+servicePort, r); err != nil {
		oplog.Err(err)
	}

}

var errorRender = func(w http.ResponseWriter, r *http.Request, httpCode int, err error) {
	render.Status(r, httpCode)
	render.JSON(w, r, map[string]interface{}{"ERROR": err.Error()})
}

func (s Serving) getUserItems(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	ctx := r.Context()
	results, err := s.Client.userItems(ctx, w, userID)
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, results)
}

func (s Serving) createUser(w http.ResponseWriter, r *http.Request) {
	userId, _ := uuid.NewRandom()
	userName := chi.URLParam(r, "user_name")
	ctx := r.Context()
	err := s.Client.createUser(ctx, w, userParams{userID: userId.String(), userName: userName})
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, User{
		Id:   userId.String(),
		Name: userName,
	})
}

func (s Serving) addItemToUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	itemID := chi.URLParam(r, "item_id")
	ctx := r.Context()
	err := s.Client.addItemToUser(ctx, w, userParams{userID: userID}, itemParams{itemID: itemID})
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, map[string]string{})
}

func (s Serving) pingPong(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusOK)
	render.PlainText(w, r, "Pong\n")
}
