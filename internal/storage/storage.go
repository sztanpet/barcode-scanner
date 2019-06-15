package storage

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"github.com/go-sql-driver/mysql"
	"github.com/juju/loggo"
)

// Storage persists Barcodes to disk before inserting them into a database
type Storage struct {
	ctx    context.Context
	path   string
	dsn    string
	db     *sql.DB
	istmt  *sql.Stmt
	insert chan inData

	lastProcessed time.Time
}

type inData struct {
	path string
	data Barcode
}

// Barcode represents the data to tbe inserted
type Barcode struct {
	Data      string
	Direction string
	ExtraData string
	CreatedAt time.Time
}

var logger = loggo.GetLogger("main.storage")
var pathProcessDurr = 1 * time.Minute

// TODO mysql: use ssl connections only, SET GLOBAL require_secure_transport ON
// dsn options: ?loc=UTC&parseTime=true&strict=true&timeout=1s&time_zone="+00:00"

// New expects a directory path as its argument.
// If the directory cannot be created an error is returned
func New(ctx context.Context, path, dsn string) (*Storage, error) {
	db, err := checkDSN(ctx, dsn)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(path, 0700)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		ctx:    ctx,
		path:   path,
		dsn:    dsn,
		db:     db,
		insert: make(chan inData, 2),
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stmt, err := db.PrepareContext(ctx, `
		INSERT INTO barcodes (data, direction, extradata, createdat, timestamp)
		VALUES (?, ?, ?, ?, NOW())
	`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.istmt = stmt

	go s.consumeData()

	return s, nil
}

func checkDSN(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(5 * time.Second)
	db.SetMaxIdleConns(3)
	db.SetMaxOpenConns(3)

	// Open doesn't open a connection to validate the DSN
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Insert persists the Barcode data to disk for resilience
// and then tries to insert it into the DB.
// On failure it is the callers job to call Insert again with the data.
func (s *Storage) Insert(data Barcode) error {
	// persist data to disk first
	// assumption: UnixNano() will give us a safely unique and nicely sortable filename
	dp := filepath.Join(s.path, strconv.FormatInt(time.Now().UnixNano(), 10))
	err := file.Serialize(dp, &data)

	// try to send the data up to the DB asap, on success the serialized file will be deleted
	select {
	case <-s.ctx.Done():
		// was the context already cancelled?
		return err
	case s.insert <- inData{path: dp, data: data}:
	default:
		logger.Infof("Insert: sending on storage.insert would have blocked, message dropped")
	}
	return err
}

// consumeData listens on the Storage.insert channel for things to insert.
// If successfull, it tries to remove the persisted data file.
// It regularly processes any persisted data files and tries to insert them.
func (s *Storage) consumeData() {
	t := time.NewTicker(pathProcessDurr)
	var runWhenIdle bool

	for {
		select {
		case <-s.ctx.Done():
			logger.Infof("consumeData: context cancelled, exiting")
			return

		case in := <-s.insert:
			err := s.dbInsert(in.data)

			// if the database insert was successfull, we can safely remove the local backup of the data
			if err == nil {
				err = os.Remove(in.path)
				if err != nil {
					// not doing anything more than logging the error will not cause trouble
					// since there is a unique index on barcode.createdat, so on re-inserting
					// we should just try and remove the file again
					logger.Errorf("Failed to remove path: %v error was: %v", in.path, err)
				}
			}

		case <-t.C:
			runWhenIdle = s.processPath(runWhenIdle)
		default:
			if runWhenIdle {
				runWhenIdle = s.processPath(runWhenIdle)
			}
		}

	}
}

// processPath tries to retries inserting the persisted data in Storage.path.
// The return value is whether it should re-run when the loop in consumeData is idle.
// draining makes sure we dont swallow runWhenIdle when hitting the rate limit.
func (s *Storage) processPath(draining bool) (runWhenIdle bool) {
	// make sure to not run too often, once every 5 seconds should be plenty
	now := time.Now()
	if now.Before(s.lastProcessed.Add(5 * time.Second)) {
		return draining
	}
	s.lastProcessed = now

	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		logger.Errorf("processPath: listing s.path failed (%v), skipping processing", err)
		return false
	}

	logger.Tracef("processPath: number of files buffered: %v", len(files))
	for _, f := range files {
		id := inData{
			path: filepath.Join(s.path, f.Name()),
		}

		err := file.Unserialize(id.path, &id.data)
		if err != nil {
			logger.Errorf("processPath: failed unseralizing %v, error was: %v", id.path, err)
			continue
		}

		select {
		case s.insert <- id:
		default:
			// abort the processing entirely to not overwhelm insertion
			// restart on idle to drain the directory asap
			logger.Infof("processPath: blocked on insert channel for path %v, aborting", id.path)
			return true
		}
	}

	return false
}

func (s *Storage) dbInsert(row Barcode) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	// the result is irrelevant, only the error matters
	_, err := s.istmt.ExecContext(
		ctx,
		row.Data,
		row.Direction,
		row.ExtraData,
		row.CreatedAt.UnixNano(),
	)

	if err != nil {
		me, ok := err.(*mysql.MySQLError)
		if !ok {
			return err
		}

		//  ignore unique error
		// uniqe error codes from:
		// https://dev.mysql.com/doc/refman/5.7/en/server-error-reference.html
		switch me.Number {
		case 1062, 1586:
			return nil
		}

		return err
	}

	return nil
}
