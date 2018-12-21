package main

import (
	"net/http"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"time"
	"encoding/json"
	"log"
)

var redisClient *redis.Client

func getSession(w http.ResponseWriter, r *http.Request) {
	if (r.Method == "GET") {
		u, err := uuid.NewRandom()
		if (err != nil) {
			return
		}

		expiry, err := time.ParseDuration("11m")
		if (err != nil) {
			return
		}

		redisClient.Set("session." + u.String(), "1", expiry)
		json.NewEncoder(w).Encode(u)
	} else if (r.Method == "DELETE") {
		session := r.URL.Query().Get("session")
		redisClient.Del("session." + session)
	}
}

func pingSession(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")

	expiry, err := time.ParseDuration("11m")
	if (err != nil) {
		return
	}

	redisClient.Set("session." + session, "1", expiry)
}

func countSession(w http.ResponseWriter, r *http.Request) {
	stringslice := redisClient.Keys("session.*")
	sessionCount := len(stringslice.Val())
	json.NewEncoder(w).Encode(sessionCount)
}

func main() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "192.168.1.2:6379",
	})

	http.HandleFunc("/", getSession);
	http.HandleFunc("/ping", pingSession);
	http.HandleFunc("/count", countSession);
	log.Fatal(http.ListenAndServe(":8080", nil))
}
