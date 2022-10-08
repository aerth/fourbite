package main

import (
	"log"
	"net/http"
	"time"

	"go.etcd.io/bbolt"
)

var bucketName = []byte("4byte")

func main() {
	addr := "http://127.0.0.1:8081"
	db, err := bbolt.Open("4byte.dat", 0600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		log.Fatalln(err)
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		var resp []byte
		var pathkey []byte
		var err error
		err = db.View(func(tx *bbolt.Tx) error {
			resp = tx.Bucket(bucketName).Get(pathkey)
			return nil
		})
		if err != nil {
			log.Printf("db read: %v", err)
		}
		if resp != nil {
			w.Write(resp)
		}
	}
	log.Println("serving 4byte:", addr)
	http.ListenAndServe(addr, http.HandlerFunc(handler))
}
