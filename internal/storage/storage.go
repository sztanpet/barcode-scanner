package storage

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"github.com/go-sql-driver/mysql"
	"github.com/juju/loggo"
)

// Storage persists Barcodes to disk before inserting them into a database
type Storage struct {
	ctx    context.Context
	path   string // processPath iterates this dir, consuming files for insertion
	dsn    string
	db     *sql.DB
	insert chan inData

	stmtMu   sync.RWMutex
	inStmt   *sql.Stmt
	deviceid uint64

	bufMu sync.Mutex
	inBuf map[[20]byte]Barcode
}

type inData struct {
	path string
	data Barcode
}

// Barcode represents the data to tbe inserted
type Barcode struct {
	Barcode        string
	Direction      string
	CurrierService string
	CreatedAt      time.Time
}

var logger = loggo.GetLogger("main.storage")
var pathProcessDurr = 1 * time.Minute
var DeviceIDMissingErr = errors.New("deviceid not set")

// TODO mysql: use ssl connections only, SET GLOBAL require_secure_transport ON
// dsn options: ?loc=UTC&parseTime=true&strict=true&timeout=1s&time_zone="+00:00"

// New expects a directory path as its argument.
// If the directory cannot be created an error is returned
func New(ctx context.Context, cfg *config.Config) (*Storage, error) {
	path := filepath.Join(cfg.StatePath, "storage")
	// Open doesn't open a connection to validate the DSN!
	db, err := sql.Open("mysql", cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(30 * time.Second)
	db.SetMaxIdleConns(3)
	db.SetMaxOpenConns(3)

	err = os.MkdirAll(path, 0700)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		ctx:    ctx,
		path:   path,
		dsn:    cfg.DatabaseDSN,
		db:     db,
		inBuf:  map[[20]byte]Barcode{},
		insert: make(chan inData, 1),
	}

	go s.consumeData()

	return s, nil
}

// TestConnection can be used to test whether the provided DSN actually works
// and to make sure the connection to the database is alive
func (s *Storage) TestConnection() error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	return s.db.PingContext(ctx)
}

func (s *Storage) pathForBarcode(data Barcode) string {
	return filepath.Join(s.path, strconv.FormatInt(data.CreatedAt.UnixNano(), 10))
}

// Insert persists the Barcode data to disk for resilience
// and tries to insert it into the DB.
func (s *Storage) Insert(data Barcode) {
	if data.CreatedAt.IsZero() {
		panic("Barcode.CreatedAt cannot be zero")
	}

	// persist data to disk first
	// assumption: UnixNano() will give us a safely unique and nicely sortable filename
	dp := s.pathForBarcode(data)
	if err := file.Serialize(dp, &data); err != nil {
		logger.Errorf("serialize failed: %v", err)
	}

	// insert the data into an in-memory buffer of Barcodes too, to protect against the case where:
	// - persisting fails and inserting fails
	// - persisting fails and insert channel would block
	// - recognize if the assumption about UnixNano does not hold
	//
	// this might cause memory exhaustion but at least we tried our best to not loose data
	s.bufMu.Lock()
	ix := sha1.Sum([]byte(dp))
	if _, ok := s.inBuf[ix]; ok {
		panic("duplicate index found, assumption does not hold")
	}
	s.inBuf[ix] = data
	s.bufMu.Unlock()

	// try to send the data up to the DB asap, on success the serialized file will be deleted
	select {
	case <-s.ctx.Done():
		// was the context already cancelled?
	case s.insert <- inData{path: dp, data: data}:
	default:
		logger.Tracef("select would have blocked")
	}
}

// consumeData listens on the Storage.insert channel for things to insert.
// If successfull, it tries to remove the persisted data file.
// It regularly processes any persisted data files and tries to insert them.
func (s *Storage) consumeData() {
	t := time.NewTicker(pathProcessDurr)
	var cancel context.CancelFunc
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

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
					//
					// the same data can be sent multiple times because:
					//  - new barcode, gets persisted, gets buffered, tries to be inserted BUT
					//    there is no internet, insertion failes, the data remains persisted and buffered
					//  - internet comes back
					//  - buffer handling runs, inserts data, removes persisted file
					//  - persisted file handling runs at near the same time, reads persisted data
					//    inserts (but unique index ignores it)
					//    tries to remove persisted file, but buffer handling already removed it
					logger.Debugf("Failed to remove path: %v error was: %v", in.path, err)
				}

				// delete from in-memory buffer of barcodes
				s.bufMu.Lock()
				ix := sha1.Sum([]byte(in.path))
				delete(s.inBuf, ix)
				s.bufMu.Unlock()

				logger.Tracef("inserted barcode: %v", in.path)
			} else {
				// otherwise just ignore the error, processPath and processBuf will retry the insert later
				logger.Debugf("dbInsert error: %v", err)
			}
		case <-t.C:
			if cancel != nil {
				cancel()
				cancel = nil
			}
			var ctx context.Context
			ctx, cancel = context.WithCancel(s.ctx)
			go func() {
				s.processBuf(ctx)
				s.processPath(ctx)
			}()
		}
	}
}

func (s *Storage) processBuf(ctx context.Context) {
	s.bufMu.Lock()
	now := time.Now()
	var toInsert []inData
	for _, data := range s.inBuf {
		if diff := now.Sub(data.CreatedAt); diff < 0 || diff < time.Second {
			continue
		}

		toInsert = append(toInsert, inData{
			path: s.pathForBarcode(data),
			data: data,
		})
	}
	s.bufMu.Unlock()

	if len(toInsert) == 0 {
		return
	}

	logger.Tracef("number of barcodes buffered: %v", len(toInsert))
	for _, in := range toInsert {
		select {
		case <-ctx.Done():
			return
		case s.insert <- in:
		}
	}
}

// processPath tries to retries inserting the persisted data in Storage.path.
// The return value is whether it should re-run when the loop in consumeData is idle.
// draining makes sure we dont swallow runWhenIdle when hitting the rate limit.
func (s *Storage) processPath(ctx context.Context) {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		logger.Errorf("listing s.path failed (%v), skipping processing", err)
		return
	}

	if len(files) > 0 {
		logger.Tracef("number of files to insert: %v", len(files))
	}

	for _, f := range files {
		id := inData{
			path: filepath.Join(s.path, f.Name()),
		}

		err := file.Unserialize(id.path, &id.data)
		if err != nil {
			logger.Errorf("failed unseralizing %v, error was: %v", id.path, err)
			continue
		}

		select {
		case <-ctx.Done():
			return
		case s.insert <- id:
		}
	}
}

func (s *Storage) dbInsert(row Barcode) error {
	err := s.ensureDeviceID()
	if err != nil {
		return err
	}
	err = s.ensureStatement()
	if err != nil {
		return err
	}

	s.stmtMu.RLock()
	defer s.stmtMu.RUnlock()

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	// the result is irrelevant, only the error matters
	_, err = s.inStmt.ExecContext(
		ctx,
		s.deviceid,
		row.Barcode,
		row.Direction,
		row.CurrierService,
		row.CreatedAt.UnixNano(),
	)

	if err != nil && !isUniqueSqlError(err) {
		return err
	}

	return nil
}

func isUniqueSqlError(err error) bool {
	me, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}

	//  ignore unique error
	// uniqe error codes from:
	// https://dev.mysql.com/doc/refman/5.7/en/server-error-reference.html
	switch me.Number {
	case 1062, 1586:
		return true
	}

	return false
}
func (s *Storage) ensureStatement() error {
	// take read lock first to check if inStmt is nil or not
	// and if it is, take a write lock to set it
	s.stmtMu.RLock()
	if s.inStmt != nil {
		s.stmtMu.RUnlock()
		return nil
	}
	s.stmtMu.RUnlock()

	// db.Stmt is safe to use concurrently, but it is not safe
	// for us to modify the pointer pointing to it concurrently
	s.stmtMu.Lock()
	defer s.stmtMu.Unlock()

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO barcodes (deviceid, barcode, direction, currier_service, created_at, timestamp)
		VALUES (?, ?, ?, ?, ?, NOW())
	`)
	if err != nil {
		return err
	}
	s.inStmt = stmt

	return nil
}
func (s *Storage) ensureDeviceID() error {
	s.stmtMu.RLock()
	defer s.stmtMu.RUnlock()

	if s.deviceid == 0 {
		return DeviceIDMissingErr
	}

	return nil
}

func (s *Storage) SetupDevice(cfg *config.Config) (did uint64, err error) {
	// is the deviceid cached?
	dip := filepath.Join(cfg.StatePath, "deviceid")
	if file.Exists(dip) {
		if err := file.Unserialize(dip, &did); err != nil {
			return 0, err
		}

		return
	}

	// not cached, try to generate it
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO devices (machineid, created_at)
		VALUES (?, NOW())
	`)
	if err != nil {
		return
	}

	mid, err := ioutil.ReadFile("/etc/machine-id")
	if err != nil {
		return
	}
	mid = bytes.TrimSpace(mid)
	if len(mid) != 32 {
		panic("invalid contents of /etc/machine-id: " + string(mid))
	}

	_, err = stmt.ExecContext(ctx, string(mid))
	if !isUniqueSqlError(err) {
		return
	}

	stmt, err = s.db.PrepareContext(ctx, `
		SELECT id
		FROM devices
		WHERE machine_id = ?
		LIMIT 1
	`)
	if err != nil {
		return
	}

	rows, err := stmt.QueryContext(ctx, string(mid))
	if err != nil {
		return
	}

	s.stmtMu.Lock()
	defer s.stmtMu.Unlock()
	if !rows.Next() {
		panic("did not find deviceid for machineid: " + string(mid))
	}

	err = rows.Scan(&did)
	if err != nil {
		return
	}

	err = file.Serialize(dip, did)
	return
}
