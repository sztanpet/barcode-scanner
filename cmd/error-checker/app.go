package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
)

func (a *app) handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func(c chan os.Signal) {
		s := <-c
		logger.Warningf("Caught signal: %v, exiting", s)
		a.exit()
	}(c)
}

func (a *app) handleLogs(binaries []string) {
	updpath, err := os.Executable()
	if err != nil {
		logger.Criticalf("os.Executable err: %v", err)
		panic(err.Error())
	}

	binPath := filepath.Join(filepath.Dir(updpath), a.bin)
	for _, bin := range binaries {
		a.handleLog(bin)
		a.handleOutput(binPath, bin)
	}
}

func (a *app) handleLog(bin string) {
	lp := filepath.Join(a.cfg.StatePath, bin+".log")
	filename := bin + time.Now().Format("_20060102_150405") + ".log.zip"
	a.sendAndTruncateFile(lp, filename)
}

func (a *app) handleOutput(binPath, bin string) {
	op := filepath.Join(binPath, bin+".output")
	filename := bin + time.Now().Format("_20060102_150405") + ".out.zip"
	a.sendAndTruncateFile(op, filename)
}

func (a *app) sendAndTruncateFile(path, filename string) {
	if !file.Exists(path) || file.Empty(path) {
		logger.Tracef("log was empty: %v", path)
		return
	}
	logger.Infof("zipping and sending log: %v", path)

	f, err := os.Open(path)
	if err != nil {
		logger.Warningf("could not pathen log %v, error was: %v", path, err)
		return
	}
	defer f.Close()

	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	zf, err := w.Create(filepath.Base(path))
	if err != nil {
		logger.Warningf("could not create zip file for log %v, error was: %v", path, err)
		return
	}

	_, err = io.Copy(zf, f)
	if err != nil {
		logger.Warningf("could not cpathy log %v, error was: %v", path, err)
		return
	}
	err = w.Close()
	if err != nil {
		logger.Warningf("could not close zip for log %v, error was: %v", path, err)
		return
	}

	err = a.bot.SendFile(buf.Bytes(), filename, true)
	if err != nil {
		logger.Warningf("sending file failed: %v", err)
		return
	}

	err = os.Truncate(path, 0)
	if err != nil {
		logger.Warningf("truncating log %v failed: %v", path, err)
		return
	}
}

func (a *app) handleServiceError() {
	// https://www.freedesktop.org/software/systemd/man/systemd.exec.html#%24EXIT_CODE
	// $EXIT_CODE is one of "exited", "killed", "dumped"
	// $SERVICE_RESULT:
	//    "success", "protocol", "timeout", "exit-code",
	//    "signal", "core-dump", "watchdog", "start-limit-hit", "resources"
	// $EXIT_STATUS: 0-255, or signal name

	exitCode := os.Getenv("EXIT_CODE")
	exitStatus := os.Getenv("EXIT_STATUS")
	srvResult := os.Getenv("SERVICE_RESULT")

	// exitStatus containes the exit code
	logger.Infof("%v exited (code: %v - status: %v - result: %v)", a.bin, exitCode, exitStatus, srvResult)
	if exitStatus == "0" && srvResult == "success" {
		logger.Tracef("no error detected with binary %v", a.bin)
		return
	}

	logger.Infof("blacklisting update for %v", a.bin)
	updpath, err := os.Executable()
	if err != nil {
		logger.Criticalf("os.Executable err: %v", err)
		panic(err.Error())
	}

	binPath := filepath.Join(filepath.Dir(updpath), a.bin)
	err = update.BlacklistUpdate(binPath, a.cfg.StatePath)
	if err != nil {
		logger.Warningf("could not blacklist update: %v", err)
		return
	}

	b, err := update.NewBinary(binPath, a.cfg)
	if err != nil {
		logger.Warningf("could not init update: %v", err)
		return
	}

	err = b.RestoreToBackup()
	if err != nil {
		logger.Warningf("could not restore update: %v", err)
		return
	}

	logger.Infof("restored backup for binary: %v", a.bin)
}
