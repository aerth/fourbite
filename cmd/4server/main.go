package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"go.etcd.io/bbolt"
)

var bucketName = []byte("4byte")

type Config struct {
	Addr           string
	DB             string
	AllowedOrigins string
	WebPrefix      string
}

func getPathKeys(ss []string) [][]byte {
	l := len(ss)
	pathkeys := make([][]byte, l)
	for i := range ss {
		if len(ss[i]) != 8 {
			log.Printf("bad path %q", ss[i])
			return nil
		}
		pathkey, err := hex.DecodeString(ss[i])
		if err != nil {
			log.Printf("error decoding hex path %q: %v", ss[i], err)
			return nil
		}
		pathkeys[i] = pathkey
	}
	return pathkeys
}

func cleanua(ua string) string {
	if len(ua) > 200 {
		ua = ua[:200]
	}
	ua = strings.TrimSpace(ua)
	for i := 0; i < len(ua); i++ {
		if !unicode.IsPrint(rune(ua[i])) {
			ua = ua[:i]
			break
		}
	}
	if ua == "" {
		return "unknown"
	}
	return ua
}

func main() {
	var config = Config{
		Addr:           "127.0.0.1:8081",
		DB:             "4byte.dat",
		AllowedOrigins: "*",
		WebPrefix:      "/4byte/",
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.StringVar(&config.Addr, "addr", config.Addr, "serve addr")
	flag.StringVar(&config.DB, "db", config.DB, "db file")
	flag.StringVar(&config.WebPrefix, "web", config.WebPrefix, "url prefix to cut (ex: /api/4byte/)")
	flag.StringVar(&config.AllowedOrigins, "origins", config.AllowedOrigins, "cors allowed origins header")
	flag.Parse()
	log.Println("opening db:", config.DB)
	db, err := bbolt.Open(config.DB, 0400, &bbolt.Options{Timeout: time.Second, ReadOnly: true})
	if err != nil {
		log.Fatalln(err)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.Header().Set("X-Info-Fourbite", "https://github.com/aerth")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache")
		log.Printf("request %s %q (%s)", r.Method, r.URL.Path, cleanua(r.UserAgent()))
		if r.Method != http.MethodGet {
			log.Printf("bad method %q", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var path = strings.TrimPrefix(r.URL.Path, config.WebPrefix)
		if len(path) < 8 {
			log.Printf("path too short %q", path)
			http.Error(w, "path too short", http.StatusBadRequest)
			return
		}
		// log.Printf("path: %q", path)
		var l = len(path)
		if l > 88 {
			http.Error(w, "path too long", http.StatusBadRequest)
			return
		}
		ss := strings.Split(path, ",")
		l = len(ss)
		if l == 0 || ss[0] == "" {
			log.Printf("bad path %q", path)
			http.NotFound(w, r)
			return
		}
		resp := make([]string, l)

		pathkeys := getPathKeys(ss)
		if len(pathkeys) == 0 {
			log.Printf("bad pathkeys %q", path)
			http.NotFound(w, r)
			return
		}
		err = db.View(func(tx *bbolt.Tx) error {
			bucket := tx.Bucket(bucketName)
			if bucket == nil {
				return ErrBucketNil
			}
			isempty := true
			for i := range pathkeys {
				resp[i] = string(bucket.Get(pathkeys[i])) // possibly empty
				if resp[i] != "" {
					isempty = false
				}
				// log.Printf("got path %d: %02x %q", i, pathkeys[i], resp[i])
			}
			if isempty {
				return ErrNotFound
			}
			return nil
		})
		if err == ErrNotFound {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			log.Printf("db read: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(resp) != 0 {
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}
	log.Println("serving 4byte:", config)
	if err := http.ListenAndServe(config.Addr, http.HandlerFunc(handler)); err != nil {
		log.Fatalln(err)
	}
}

var ErrBucketNil = errors.New("bucket is nil")
var ErrNotFound = errors.New("not found")
