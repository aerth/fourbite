package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gitlab.com/aquachain/aquachain/crypto"
	"go.etcd.io/bbolt"
)

type Config struct {
	DB      string
	Verbose bool
}

func main() {
	log.SetFlags(0)
	var config = Config{
		DB:      "/var/lib/4byte.dat",
		Verbose: false,
	}

	flag.StringVar(&config.DB, "db", config.DB, "db file")
	flag.BoolVar(&config.Verbose, "v", config.Verbose, "verbose output to stderr")
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
			if verbose {
				log.Println("hashing:", arg)
			}
			b := crypto.Keccak256Hash([]byte(arg))
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
		resp, err := lookup4byte(db, b)
		if err != nil {
			return err
		}
		// print out
		fmt.Fprintf(os.Stdout, "%s %s\n", arg, resp)
	}
	return nil
}

func lookup4byte(db *bbolt.DB, b []byte) (string, error) {

	var resp string
	// db read
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("4byte")).Get(b)
		if b != nil {
			resp = string(b)
			return nil
		}
		resp = "not found"
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("db %v", err)
	}
	return resp, nil
}
