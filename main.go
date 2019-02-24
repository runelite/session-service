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

func init() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(404)
			return
		}

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
	countResponse := []byte{'0'}
	go func() {
		ticker := time.NewTicker(time.Minute).C
	outer:
		for {
			var cursor uint64
			var count int

			for {
				var keys []string
				var err error
				keys, cursor, err = redisClient.Scan(cursor, "session.*", 1000).Result()
				if err != nil {
					log.Printf("error scanning: %v\n", err)
					time.Sleep(time.Second * 30)
					continue outer
				}

				count += len(keys)
				if cursor == 0 {
					break
				}
			}

			newRes, err := json.Marshal(count)
			if err != nil {
				panic(err)
			}
			countResponse = newRes
			<-ticker
		}
	}()
	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		w.Write(countResponse)
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
