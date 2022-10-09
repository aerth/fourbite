package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"go.etcd.io/bbolt"
)

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalln("fatal:", err)
	}
}

func newWalkFn(tx *bbolt.Tx) func(string, fs.DirEntry, error) error {
	bucket, err := tx.CreateBucketIfNotExists([]byte("4byte"))
	if err != nil {
		panic(err)
	}
	_ = bucket
	return func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		spath := d.Name()
		ok, err := regexp.Match("^[a-z0-9]{8}$", []byte(spath))
		if err != nil {
			return fmt.Errorf("invalid path, bad hex: %v", err)
		}
		if !ok {
			return fmt.Errorf("invalid path, not hex")
		}
		k, err := hex.DecodeString(spath)
		if err != nil {
			return fmt.Errorf("invalid path, invalid hex: %v", err)
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file err: %v", err)
		}
		v, err := ioutil.ReadAll(f)
		if err != nil {
			f.Close()
			return fmt.Errorf("read file err: %v", err)
		}
		f.Close()
		log.Println("saving", spath, string(v))
		return bucket.Put(k, v)
	}
}

func run() error {
	var path = "../4byte/4bytes-master/signatures"
	var dbpath = "4byte.dat"
	flag.StringVar(&path, "path", path, "path to signatures directory")
	flag.StringVar(&dbpath, "db", dbpath, "file to write new database")
	flag.Parse()
	db, err := bbolt.Open(dbpath, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bbolt.Tx) error {
		var walkfn fs.WalkDirFunc = newWalkFn(tx)
		if err := filepath.WalkDir(path, walkfn); err != nil {
			return err
		}
		return nil
	})

}
