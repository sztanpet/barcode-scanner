package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"github.com/juju/loggo"
)

const platform = runtime.GOOS + "-" + runtime.GOARCH
const blacklistDir = "blacklisted-updates"

var logger = loggo.GetLogger("main.update")
var ErrFileInvalid = errors.New("File failed integrity check")

type Binary struct {
	Name string
	// the baseURL to contact for updates
	baseURL string
	client  *http.Client
	// the binary path to update, usually os.Executable()
	path string
	// the current hash of the binary path
	hash []byte
	// base directory for blacklistDir
	statePath string
}

// info is the information about the update version
// from version.json
type info struct {
	Hash       string
	BinaryPath string
}

func NewBinary(binPath string, cfg *config.Config) (*Binary, error) {
	h, err := getFileSha(binPath)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(binPath)
	return &Binary{
		Name:      name,
		baseURL:   cfg.UpdateBaseURL + "/" + name,
		path:      binPath,
		statePath: cfg.StatePath,
		hash:      h,
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
	_ = os.Remove(b.signalFile())
}

func (b *Binary) signalFile() string {
	return filepath.Join(
		file.TmpDir,
		"UPD-"+filepath.Base(b.path),
	)
}

func (b *Binary) Check() error {
	if b.currentlyBlacklisted() {
		if err := b.RestoreToBackup(); err != nil {
			return err
		}
	}

	u := b.getURL("version.json")
	logger.Tracef("Checking url: %v", u)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Errorf("Response statuscode for url %v was %v", u, resp.StatusCode)
		return fmt.Errorf("Non-200 response code (%v)", u)
	}

	nfo := &info{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(nfo)
	if err != nil {
		logger.Errorf("Invalid json body in response: %v", err)
		return err
	}

	if nfo.Hash == hex.EncodeToString(b.hash) {
		logger.Tracef("No new update found (%v)", u)
		return nil
	}

	if b.backupBlacklisted(nfo.Hash) {
		logger.Tracef("Update blacklisted %v (%v)", nfo.Hash, u)
		// nothing to update, not an error, do not want to make it distinguishable
		return nil
	}

	return b.download(nfo)
}

func (b *Binary) currentlyBlacklisted() bool {
	return b.backupBlacklisted(hex.EncodeToString(b.hash))
}

func (b *Binary) RestoreToBackup() error {
	// rename binary.bkup to binary, delete binary.bkup, mark that the binary should restart
	bkupPath := b.path + ".bkup"
	if !file.Exists(bkupPath) {
		logger.Infof("backup does not exist, not restoring (%v)", bkupPath)
		return nil
	}

	bf, err := os.Open(bkupPath)
	if err != nil {
		logger.Infof("could not open backup: %v", err)
		return nil
	}

	err = file.WriteAtomically(b.path, bf)
	_ = bf.Close()

	if err != nil {
		logger.Warningf("could not restore backup: %v", err)
		return err
	}

	err = os.Remove(bkupPath)
	if err != nil {
		logger.Warningf("could not delete backup: %v", err)
		return nil
	}

	sf, err := os.Create(b.signalFile())
	if err != nil {
		logger.Warningf("could not create signalfile: %v", err)
	}

	_ = sf.Close()
	logger.Infof("restored to backup")
	return nil
}

// domain.tld/<binary-base-name>/<GOOS>-<GOARCH>/<file>
// ex: domain.tld/barcode-scanner/linux-arm/version.json
// version.json: {"hash":"abcdef123456789", "binaryPath":"barcode-scanner"}
// binary-path -> domain.tld/barcode-scanner/linux-arm/barcode-scanner
func (b *Binary) getURL(file string) string {
	v := url.Values{}
	v.Set("currentSha", hex.EncodeToString(b.hash))

	ret := b.baseURL + "/" + platform + "/" + file
	ret += "?" + v.Encode()
	return ret
}

// download loads the file from the version.json
func (b *Binary) download(nfo *info) error {
	logger.Tracef("downloading update: %#v", nfo)
	u := b.getURL(nfo.BinaryPath)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return b.update(resp.Body, nfo)
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
		logger.Errorf("corrupted download, hashes do not match: %#v", nfo)
		return ErrFileInvalid
	}

	// backup binary
	err = b.backup()
	if err != nil {
		logger.Errorf("backup of %v failed: %v", b.path, err)
		return err
	}

	// rename over the binary
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = file.WriteAtomically(b.path, f)
	if err != nil {
		logger.Errorf("overwriting binary %v failed: %v", b.path, err)
		return err
	}

	// create signal file, ignore error
	sf, _ := os.Create(b.signalFile())
	if sf != nil {
		_ = sf.Close()
	}
	return nil
}

func (b *Binary) backup() error {
	src := b.path
	dest := src + ".bkup"
	return file.CopyOver(src, dest)
}

func (b *Binary) backupBlacklisted(hash string) bool {
	fp := filepath.Join(b.statePath, blacklistDir, hash)
	return file.Exists(fp)
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

// BlacklistUpdate marks binPath as an invalid update
// by creating a marker file under statePath
func BlacklistUpdate(binPath, statePath string) error {
	h, err := getFileSha(binPath)
	if err != nil {
		return err
	}

	fp := filepath.Join(statePath, blacklistDir, hex.EncodeToString(h))
	f, err := os.OpenFile(fp, os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	_ = f.Close()
	return nil
}
