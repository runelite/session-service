package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

const (
	sessionExpiry = 11 * time.Minute
)

var redisClient *redis.Client
var lastCountTime = time.Now()
var lastCount = -1

func init() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			u, err := uuid.NewRandom()
			if err != nil {
				log.Printf("unable to generate uuid: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_, err = redisClient.Set("session."+u.String(), "1", sessionExpiry).Result()
			if err != nil {
				log.Printf("unable to create new session: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(u)
		case http.MethodDelete:
			session := r.URL.Query().Get("session")
			redisClient.Del("session." + session)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func init() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		session := r.URL.Query().Get("session")
		if len(session) != 36 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, err := redisClient.Set("session."+session, "1", sessionExpiry).Result()
		if err != nil {
			log.Printf("unable to create new session: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
	})
}

func init() {
	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		if lastCount == -1 || lastCountTime.Add(time.Minute).Before(time.Now()) {
			stringslice := redisClient.Keys("session.*")
			lastCount = len(stringslice.Val())
			lastCountTime = time.Now()
		}
		json.NewEncoder(w).Encode(lastCount)
	})
}

func main() {
	listenAddr := flag.String("listenaddr", ":8081", "listen address eg :8081")
	addrPtr := flag.String("redisaddr", "127.0.0.1:6379", "redis address eg 127.0.0.1:6379")
	flag.Parse()

	redisClient = redis.NewClient(&redis.Options{
		Addr: *addrPtr,
	})

	_, err := redisClient.Ping().Result()
	if err != nil {
		log.Fatalf("unable to connect to Redis: %v\n", err)
	}

	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
