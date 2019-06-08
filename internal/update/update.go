package update

import (
	"bytes"
	"crypto/sha256"
	"io"
	"sync"
	"time"
)

type Binary struct {
	BaseURL  string
	StateDir string

	mu   sync.Mutex
	hash string
	path string
}

var UpdateDurr = 5 * time.Minute

// binary path
// => hash a current hash,
// ha elter az update serveren levotol akkor update
// => a pathbol generaljuk az url-t a baseurl-hez merten
// ha van uj update akkor azt letoltjuk, ellenorizzuk a hasht,
// es jelezni kell a binarisnak hogy induljon ujra (temp dirbe rakott signal file-al)
// ujraindulaskor a signal filet el kell tavolitani
// ugyanezt az update processnek is
func (b *Binary) Run() {
	t := time.NewTicker(UpdateDurr)
	for {
		<-t.C
		if !b.canUpdate() {
			continue
		}

	}
}

func (b *Binary) check() {
	// http request
}

func (b *Binary) canUpdate() bool {
	//
	return false
}

func (b *Binary) shouldRestart() bool {
	// van e signal file
	return false
}

func (b *Binary) Cleanup() {
	// eltavolitani a signal filet
}

func NewBinary(path string) *Binary {
	return &Binary{
		path: path,
	}
}

func shaMatches(expected []byte, actual io.Reader) (bool, error) {
	h := sha256.New()
	_, err := io.Copy(h, actual)
	if err != nil {
		return false, err
	}

	return bytes.Equal(h.Sum(nil), expected), nil
}
