package file

import (
	"archive/zip"
	"bytes"
	"encoding/gob"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

var TmpDir = filepath.Join(os.TempDir(), "barcode-scanner")

func Serialize(path string, data interface{}) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	tf, err := ioutil.TempFile(dir, filepath.Base(path))
	if err != nil {
		return err
	}

	e := gob.NewEncoder(tf)
	err = e.Encode(data)
	if err != nil {
		_ = tf.Close()
		return err
	}

	err = tf.Sync()
	if err != nil {
		_ = tf.Close()
		return err
	}
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

func WriteAtomically(dest string, r io.Reader) error {
	// cross-filesystem renames can't work, so create temp file in the same dir
	// as the destination
	tf, err := ioutil.TempFile(filepath.Dir(dest), filepath.Base(dest))
	if err != nil {
		return err
	}
	defer tf.Close()

	_, err = io.Copy(tf, r)
	if err != nil {
		return err
	}

	err = tf.Sync()
	if err != nil {
		return err
	}

	err = os.Rename(tf.Name(), dest)
	if err != nil {
		return err
	}

	return nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Empty(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.Size() == 0
}

func CopyOver(src, dest string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	return WriteAtomically(dest, sf)
}

func Append(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	return nil
}

func ZipFile(in io.Reader, filename string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	f, err := w.Create(filename)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(f, in)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func ZipBytes(in []byte, filename string) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(in)
	return ZipFile(buf, filename)
}
