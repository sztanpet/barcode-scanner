package file

import (
	"encoding/gob"
	"io/ioutil"
	"os"
	"path/filepath"
)

var TmpDir = filepath.Join(os.TempDir(), "barcode-scanner")

/*
func init() {
	TmpDir = filepath.Join(os.TempDir(), "barcode-scanner")
}
*/
func Serialize(path string, data interface{}) error {
	tf, err := ioutil.TempFile("", filepath.Base(path))
	if err != nil {
		return err
	}

	e := gob.NewEncoder(tf)
	err = e.Encode(data)
	if err != nil {
		_ = tf.Close()
		return err
	}

	tf.Sync()
	_ = tf.Close()

	err = os.Rename(tf.Name(), path)
	if err != nil {
		return err
	}

	return nil
}

func Unserialize(path string, data interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	d := gob.NewDecoder(f)
	err = d.Decode(data)
	if err != nil {
		return err
	}

	return nil
}

func ensureTmpDir() error {
	err := os.Mkdir(TmpDir, 0700)
	if os.IsExist(err) {
		return nil
	}

	return err
}

// TmpFile creates a file in the temporary directory, if it already exists
// the file is truncated, an open file handle is returned
func TmpFile(name string) (*os.File, error) {
	err := ensureTmpDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(TmpDir, name)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return f, err
	}

	return f, nil
}
