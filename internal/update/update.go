package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"code.sztanpet.net/barcode-scanner/internal/file"
	"github.com/juju/loggo"
)

const platform = runtime.GOOS + "-" + runtime.GOARCH

var UpdateDurr = 5 * time.Minute

var logger = loggo.GetLogger("main.update")
var ErrFileInvalid = errors.New("File failed integrity check")

type Binary struct {
	BaseURL  string
	StateDir string
	client   *http.Client

	mu   sync.Mutex
	hash []byte
	path string
}

// info is the information about the update version
// from version.json
type info struct {
	Hash       string
	BinaryPath string
}

func NewBinary(path string) (*Binary, error) {
	h, err := getFileSha(path)
	if err != nil {
		return nil, err
	}

	return &Binary{
		path: path,
		hash: h,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// ShouldRestart checks if the binary should restart itself
func (b *Binary) ShouldRestart() bool {
	return file.Exists(b.signalFile())
}

// Cleanup removes the update signal file if present
func (b *Binary) Cleanup() {
	// eltavolitani a signal filet
	_ = os.Remove(b.signalFile())
}

func (b *Binary) signalFile() string {
	return filepath.Join(
		file.TmpDir,
		"UPD-"+filepath.Base(b.path),
	)
}

// binary path
// => hash a current hash,
// ha elter az update serveren levotol akkor update
// => a pathbol generaljuk az url-t a baseurl-hez merten
// ha van uj update akkor azt letoltjuk, ellenorizzuk a hasht,
// es jelezni kell a binarisnak hogy induljon ujra (temp dirbe rakott signal file-al)
// ujraindulaskor a signal filet el kell tavolitani
// ugyanezt az update processnek is
func (b *Binary) UpdateLoop() {
	t := time.NewTicker(UpdateDurr)
	for {
		<-t.C
		// TODO
		b.check()
	}
}

func (b *Binary) check() {
	// http request
	u := b.getURL("version.json")

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}

	resp, err := b.client.Do(req)
	_ = resp
	// TODO
	// unmarshal version.json into *info
	// decide if we need to update
	// call .download() if so
}

// download loads the file from the version.json
func (b *Binary) download(nfo *info) {
	// http request
	u := b.getURL(nfo.BinaryPath)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}

	resp, err := b.client.Do(req)
	_ = resp
	// TODO call .update()
}

// update backs up the main binary, than downloads
// the new update from the io.Reader and than renames it
// on top of the current executable
// finally it creates the signal file
func (b *Binary) update(r io.Reader, nfo *info) error {
	// copy from reader into tempfile
	f, err := file.TmpFile(filepath.Base(b.path))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}

	// check if downloaded file matches the info
	// first, rewind
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	fh, err := getSha(f)
	if err != nil {
		return err
	}
	if nfo.Hash != hex.EncodeToString(fh) {
		return ErrFileInvalid
	}

	// backup binary
	err = b.backup()
	if err != nil {
		return err
	}

	// rename over the binary
	err = os.Rename(f.Name(), b.path)
	if err != nil {
		return err
	}

	// create signal file, ignore error
	sf, _ := os.Create(b.signalFile())
	_ = sf.Close()
	return nil
}

func (b *Binary) backup() error {
	src := b.path
	dest := src + ".bkup"
	return file.CopyOver(src, dest)
}

func (b *Binary) getURL(file string) string {
	v := url.Values{}
	v.Set("currentSha", hex.EncodeToString(b.hash))

	ret := b.BaseURL + "/" + platform + "/" + file
	ret += "?" + v.Encode()
	return ret
}

func getFileSha(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return getSha(f)
}

func getSha(r io.Reader) ([]byte, error) {
	h := sha256.New()
	_, err := io.Copy(h, r)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
