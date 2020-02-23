package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

const (
	sessionExpiry = 11 * time.Minute
	sessionKey = "session"
	loggedInKey = "loggedin"
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
			fallthrough
		case http.MethodPost:
			u, err := uuid.NewRandom()
			if err != nil {
				log.Printf("unable to generate uuid: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			err = redisClient.ZAdd(sessionKey, redis.Z{
				Score: float64(time.Now().Unix()),
				Member: u.String(),
			}).Err()
			if err != nil {
				log.Printf("unable to create new session: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(u)
		case http.MethodDelete:
			session := r.URL.Query().Get("session")
			redisClient.ZRem(sessionKey, session)
			redisClient.ZRem(loggedInKey, session)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func init() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		session := query.Get("session")
		if len(session) != 36 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err := redisClient.ZAdd(sessionKey, redis.Z{
			Score: float64(time.Now().Unix()),
			Member: session,
		}).Err()
		if err != nil {
			log.Printf("unable to create new session: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		loggedIn := query.Get("logged-in")
		if loggedIn == "true" {
			err = redisClient.ZAdd(loggedInKey, redis.Z{
				Score: float64(time.Now().Unix()),
				Member: session,
			}).Err()
			if err != nil {
				log.Printf("unable to zadd to logged in: %v\n", err)
			}
		}

		w.WriteHeader(200)
	})
}

func init() {
	countResponse := []byte{'0'}
	loggedInResponse := []byte{'0'}
	go func() {
		ticker := time.NewTicker(time.Minute).C
		for range ticker {
			result, err := redisClient.ZRangeByScore(sessionKey, redis.ZRangeBy {
				Min: strconv.Itoa(int(time.Now().Unix()) - int(sessionExpiry.Seconds())),
				Max: strconv.Itoa(int(time.Now().Unix())),
			}).Result()
			if err != nil {
				log.Printf("error running zrangebyscore: %v\n", err)
			} else {
				newRes, err := json.Marshal(len(result))
				if err != nil {
					panic(err)
				}

				countResponse = newRes
			}

			result, err = redisClient.ZRangeByScore(loggedInKey, redis.ZRangeBy {
				Min: strconv.Itoa(int(time.Now().Unix()) - int(sessionExpiry.Seconds())),
				Max: strconv.Itoa(int(time.Now().Unix())),
			}).Result()
			if err != nil {
				log.Printf("error running zrangebyscore: %v\n", err)
			} else {
				newRes, err := json.Marshal(len(result))
				if err != nil {
					panic(err)
				}

				loggedInResponse = newRes
			}
		}
	}()
	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		w.Write(countResponse)
	})
	http.HandleFunc("/count/logged-in", func(w http.ResponseWriter, r *http.Request) {
		w.Write(loggedInResponse)
	})
}

func init() {
	go func() {
		ticker := time.NewTicker(time.Minute).C
		for range ticker {
			redisClient.ZRemRangeByScore(sessionKey, "-inf", strconv.Itoa(int(time.Now().Unix()) - int(sessionExpiry.Seconds())))
			redisClient.ZRemRangeByScore(loggedInKey, "-inf", strconv.Itoa(int(time.Now().Unix()) - int(sessionExpiry.Seconds())))
		}
	}()
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
