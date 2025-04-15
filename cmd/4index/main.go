package main

import (
	"archive/zip"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalln("fatal:", err)
	}
}

type fourbite struct {
	b [4]byte
	s string
}

var ctx = context.TODO()

func createBuckets(tx *bbolt.Tx) error {
	_, err := tx.CreateBucketIfNotExists([]byte("4byte"))
	if err != nil {
		return fmt.Errorf("create bucket: %v", err)
	}
	return nil
}

type fourbytes []fourbite

func (f fourbytes) Len() int {
	return len(f)
}
func (f fourbytes) Less(i, j int) bool {
	return f[i].b[0] < f[j].b[0]
}
func (f fourbytes) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
func (f fourbite) String() string {
	return fmt.Sprintf("%s (0x%02x)", f.s, f.b[:])
}
func (f fourbytes) String() string {
	var s string
	l := len(f)
	for _, v := range f[:l] {
		s += v.String() + ", "
	}
	if l > 0 {
		s = s[:len(s)-1]
	}
	return s
}
func startIt(wgdone func(), filesystem fs.FS, filenames []fs.DirEntry, zipfiles []*zip.File, isDir bool, innerdir string, ch chan fourbite) {
	x := len(filenames) - 1
	defer wgdone()
	defer close(ch)
	if !isDir {
		log.Printf("zip dir %s", innerdir)
		for _, f := range zipfiles {
			matched := strings.HasPrefix(f.Name, innerdir)
			if !matched {
				continue
			}
			name := strings.TrimPrefix(f.Name, innerdir)
			if name == "" {
				continue // first dir
			}
			if len(name) != 8 {
				println("error: file name is not 8 bytes:", name)
				continue
			}
			rdr, err := f.Open()
			if err != nil {
				println("error opening file:", name, err)
				continue
			}
			got, err := io.ReadAll(rdr)
			rdr.Close()
			if err != nil {
				println("error reading file:", name, err)
				continue
			}
			l := len(got)
			if l == 0 {
				println("error: file is empty:", name)
				continue
			}
			if got[l-1] != ')' { // filter out junk
				println("error: file is not valid:", name, string(got))

				continue
			}
			pack := fourbite{s: string(got)}
			if _, err := hex.Decode(pack.b[:], []byte(name)); err != nil {
				println("error decoding name:", name, err)
				continue
			}
			if !sendPack(ch, pack) {
				log.Printf("error sending pack: %s", pack.String())
				return
			}
		}
		return
	}
	log.Printf("Is dir")
	for i := 0; i < len(filenames); i++ {
		f := filenames[x-i] // reverse order (so zip's Open speeds up, otherwise it gradually slows down)
		name := f.Name()    // eg: 0abcdef8

		gotfile, err := filesystem.Open(filepath.Join(innerdir, name))
		if err != nil {
			println("error opening file:", name, err)
			continue
		}
		got, err := io.ReadAll(gotfile)
		gotfile.Close()
		if err != nil {
			println("error reading file:", name, err)
			continue
		}
		// println("worker", thrid, "got", name, string(got))
		ch <- fourbite{b: [4]byte{name[0], name[1], name[2], name[3]}, s: string(got)}
	}
}

func run() error {
	var path = "4bytes-master.zip"
	var dbpath = "/tmp/4byte.dat" // faster in tmp
	var t1 = time.Now()
	flag.StringVar(&path, "path", path, "path to directory/zip containing 4bytes repo")
	flag.StringVar(&dbpath, "db", dbpath, "file to write 4bytes entries (bbolt db)")
	flag.Parse()
	if strings.Contains(path, "signatures") {
		println("Warning: path contains 'signatures', use short path")
	}
	// check path
	f, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat: %v", err)
	}
	isDir := f.IsDir()
	println("path exists:", path, "is dir:", isDir)
	// open db
	db, err := bbolt.Open(dbpath, 0600, &bbolt.Options{
		Timeout:        1 * time.Second,
		NoFreelistSync: true,
	},
	)
	if err != nil {
		return err
	}
	defer db.Close()

	// create bucket
	err = db.Update(createBuckets)
	if err != nil {
		return fmt.Errorf("create bucket: %v", err)
	}

	var zipfiles []*zip.File    // for zip
	var filenames []fs.DirEntry // for zip and dir

	// zip or dir
	var filesystem fs.FS
	var innerdir = "."
	var mode = "dir"
	if f.IsDir() {
		log.Printf("path: %s is dir", path)
		if _, err := os.Stat(filepath.Join(path, "4bytes-master", "signatures", "0000000c")); err == nil {
			path = filepath.Join(path, "4bytes-master", "signatures")
		} else if _, err := os.Stat(filepath.Join(path, "signatures", "0000000c")); err == nil {
			path = filepath.Join(path, "signatures")
		} else if _, err := os.Stat(filepath.Join(path, "0000000c")); err == nil {
			// ok already
		} else {
			return fmt.Errorf("path %q is not 4bytes repo", path)
		}
		filesystem = os.DirFS(path)
	} else { // is zip
		log.Println("path is zip:", path)
		r, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer r.Close()

		zipfiles = r.Reader.File
		if len(zipfiles) == 0 {
			return fmt.Errorf("zip is empty")
		}
		filesystem = ZipFS{Reader: &r.Reader}
		if _, err := filesystem.Open("4bytes-master/signatures/23b872dd"); err != nil {
			return fmt.Errorf("zip doesnt look right: %v", err)
		}
		innerdir = "4bytes-master/signatures/"
		mode = "zip"
	}
	println("going to walk", mode, "filesystem")
	ch := make(chan fourbite, 100)

	filenames, err = fs.ReadDir(filesystem, innerdir)
	if err != nil {
		return fmt.Errorf("read dir: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go startIt(wg.Done, filesystem, filenames, zipfiles, isDir, innerdir, ch)
	const bufSize = 10000
	var buf = make(fourbytes, bufSize)
	var current = 0
	var totalSaved = 0
	var totalNumber = len(filenames) // ESTIMATE. some could be filtered out

Loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case fb, moreRemaining := <-ch: // very fast incoming
			if moreRemaining && fb.s == "" {
				continue
			}
			if moreRemaining {
				buf[current] = fb
				current++
			}
			if (!moreRemaining && current != 0) || current == bufSize {
				// log.Printf("saving chunk: %d/%d more=%v (%02x = %s)", current, bufSize, moreRemaining, buf[0].b, buf[0].s)
				err = saveChunkToDb(db, buf[:current])
				if err != nil {
					return fmt.Errorf("db update err: %v", err)
				}
				if true {
					totalSaved += current
					remaining := totalNumber - totalSaved
					avg := float64(totalSaved) / time.Since(t1).Seconds()
					log.Printf("total: %d, avg: %f sig/sec", totalSaved, avg)
					if avg == 0 {
						avg = 0.0001 // avoid div by 0
					}
					log.Printf("estimated remaining time: %.2f sec (%d sigs)", (float64(remaining) / (avg)), remaining)
				}
				current = 0 // reset
			}
			if !moreRemaining { // chan closed
				log.Printf("done with %d signatures", totalSaved)
				break Loop
			}
		}
	}
	println("waiting")
	wg.Wait()
	log.Printf("done after %s", time.Since(t1).String())
	return db.Sync()
}

func saveChunkToDb(db *bbolt.DB, chunk fourbytes) error {
	err := db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("4byte"))
		var err error
		for i := range chunk {
			err = bucket.Put(chunk[i].b[:], []byte(chunk[i].s))
			if err != nil {
				log.Printf("error putting %s: %v", chunk[i].b, err)
				return err
			}
		}
		return err
	})
	if err != nil {
		return err
	}
	first := chunk[0].b[:]
	last := chunk[len(chunk)-1].b[:]
	log.Printf("saved %d entries to db from %02x to %02x", len(chunk), first, last)
	return nil
}

type zipFileWrapper struct {
	io.ReadCloser
	fileInfo fs.FileInfo
}

func (z *zipFileWrapper) Stat() (fs.FileInfo, error) {
	return z.fileInfo, nil
}

func (z *zipFileWrapper) Read(b []byte) (int, error) {
	return z.ReadCloser.Read(b)
}

func wrapZipFile(f *zip.File) (*zipFileWrapper, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	return &zipFileWrapper{
		ReadCloser: rc,
		fileInfo:   f.FileInfo(),
	}, nil
}

type ZipFS struct {
	*zip.Reader
}

func noopwrapZipFile(f *zip.File) (fs.File, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	return &zipFileWrapper{
		ReadCloser: rc,
		fileInfo:   nil,
	}, nil
}

func (zfs ZipFS) Open(name string) (fs.File, error) {
	for _, f := range zfs.File {
		if f.Name == name {
			// return f.Open()
			return noopwrapZipFile(f)
		}
	}
	return nil, fs.ErrNotExist
}

func (zfs ZipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	// println("zipfs: ReadDir", name)
	t1 := time.Now()
	if name == "./4bytes-master/signatures/" {
		name = "4bytes-master/signatures/"
	}
	if name != "4bytes-master/signatures/" {
		return nil, fs.ErrNotExist
	}
	var entries []fs.DirEntry
	for _, f := range zfs.File {
		// println("zipfs found file ", f.Name)
		if !strings.HasPrefix(f.Name, "4bytes-master/signatures/") {
			continue
		}
		if f.Name != name && fs.ValidPath(f.Name) {
			entries = append(entries, fs.FileInfoToDirEntry(f.FileInfo()))
		} else if f.Name != name {
			return nil, fmt.Errorf("invalid path: %s", f.Name)
		}
	}
	if len(entries) == 0 {
		println("Name: ", name)
		return nil, fs.ErrNotExist
	}
	log.Printf("zipfs: found %v entries in %s", len(entries), time.Since(t1)) // found 916173 entries in 77.99895ms
	return entries, nil
}

func sendPack(ch chan fourbite, pack fourbite) bool {
	select {
	case ch <- pack:
		return true
	case <-ctx.Done():
		return false
	}
}
