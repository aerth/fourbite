package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"log"
	"os"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/sha3"
)

type Config struct {
	DB      string
	Verbose bool
	Fatal   bool
}

func main() {
	log.SetFlags(0)
	var config = Config{
		DB:      "/var/lib/4byte.dat",
		Verbose: false,
		Fatal:   false,
	}

	flag.StringVar(&config.DB, "db", config.DB, "db file")
	flag.BoolVar(&config.Verbose, "v", config.Verbose, "verbose output to stderr")
	flag.BoolVar(&config.Fatal, "f", config.Fatal, "treat not found as fatal error")
	flag.Parse()
	if err := run(config); err != nil {
		log.Fatalln("fatal:", err)
	}
}
func run(config Config) error {
	verbose := config.Verbose
	if verbose {

	}
	log.Println("opening db:", config.DB)
	db, err := bbolt.Open(config.DB, 0400, &bbolt.Options{Timeout: time.Second, ReadOnly: true})
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()
	var args []string = flag.Args()
	for i, arg := range args {
		// decode in
		arg = strings.TrimSpace(arg)
		l := len(arg)
		if l == 0 {
			continue
		}
		// hash
		if arg[l-1] == ')' {
			b := Keccak256([]byte(arg))
			fmt.Fprintf(os.Stdout, "%s %#02x\n", arg, b[:4])
			continue
		}
		if verbose {
			log.Println("unhashing:", arg)
		}
		// unhash
		start := 0
		if len(arg) > 2 && arg[0] == '0' && arg[1] == 'x' {
			start = 2
		}
		b, err := hex.DecodeString(arg[start:])
		if err != nil {
			return fmt.Errorf("bad input %v", err)
		}
		if len(b) != 4 {
			return fmt.Errorf("argument %d is not four bytes, %d != 4", i, len(b))
		}
		resp, err := lookup4byte(db, b, config.Fatal)
		if err != nil {
			return err
		}
		// print out
		fmt.Fprintf(os.Stdout, "%s %s\n", arg, resp)
	}
	return nil
}

// db read
func lookup4byte(db *bbolt.DB, key []byte, failIfNotFound bool) (string, error) {
	var resp string
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("4byte")).Get(key)
		if b != nil {
			resp = string(b)
			return nil
		}
		if failIfNotFound {
			return fmt.Errorf("%#02x not found", key)
		}
		resp = "not found"
		return nil
	})
	return resp, err
}

func Keccak256(data ...[]byte) []byte {
	d := NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}
func NewKeccak256() hash.Hash {
	return sha3.NewLegacyKeccak256()
}
