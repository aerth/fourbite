package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

var bucketName = []byte("4byte")

type Config struct {
	Addr           string
	DB             string
	AllowedOrigins string
	WebPrefix      string
}

func main() {
	var config = Config{
		Addr:           "127.0.0.1:8081",
		DB:             "4byte.dat",
		AllowedOrigins: "*",
		WebPrefix:      "/4byte/",
	}

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
		if r.Method != http.MethodGet {
			return
		}

		var path = strings.TrimPrefix(r.URL.Path, config.WebPrefix)
		if path == "" {
			return
		}
		path = path[1:]
		var l = len(path)
		if l > 800 {
			log.Println("over 800")
			path = path[:899]
			return
		}
		if l != 8 && (l+1)%9 != 0 {
			http.NotFound(w, r)
			return
		}

		ss := strings.Split(path, ",")
		l = len(ss)
		if l == 0 || ss[0] == "" {
			return
		}
		resp := make([]string, l)
		pathkeys := make([][]byte, l)
		for i := 0; i < l; i++ {
			pathkey, err := hex.DecodeString(ss[i])
			if err != nil {
				fmt.Fprintf(w, "error %v", err)
				return
			}
			log.Printf("loading path %#02x", pathkey)
			pathkeys[i] = pathkey

		}
		err = db.View(func(tx *bbolt.Tx) error {
			bucket := tx.Bucket(bucketName)
			if bucket == nil {
				return fmt.Errorf("bucket is nil...")
			}
			for i := 0; i < l; i++ {
				resp[i] = string(bucket.Get(pathkeys[i]))
				log.Printf("%d: %s", i, resp[i])
			}
			return nil
		})
		if err != nil {
			log.Printf("db read: %v", err)
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
