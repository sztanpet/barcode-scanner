// +build !windows
// +build !plan9

// status implements monitoring of system status
// checks for new dmesg output and logs it
// checks for any error output from binaries
// checks for system temperature, uptime, and load
// dmesg and error output is zipped and sent via telegram
package status

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("main.status")

type Status struct {
	ctx context.Context
	bot *telegram.Bot
}

func New(ctx context.Context, bot *telegram.Bot) *Status {
	return &Status{
		ctx: ctx,
		bot: bot,
	}
}

func (s *Status) Check() {
	s.sysinfo()
	s.dmesg()
}

func (s *Status) dmesg() {
	cmd := exec.CommandContext(s.ctx, "dmesg", "-e", "-c")
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("dmesg -e -c failed: %v", err)
		return
	}

	if len(out) == 0 {
		logger.Tracef("dmesg output was empty")
		return
	}

	// zip it
	filename := time.Now().Format("20060102_150405") + "_dmesg.txt"
	buf, err := s.zip(out, filename)
	if err != nil {
		logger.Warningf("zipping file failed: %v", err)
	}
	err = s.bot.SendFile(buf.Bytes(), filename+".zip", true)
	if err != nil {
		logger.Warningf("sending file failed: %v", err)
	}
}

func (s *Status) sysinfo() {
	var temp float64
	{
		// $ cat raspitemp.sh
		// cat /sys/class/thermal/thermal_zone0/temp
		// 43802 -> 43.802C
		t, err := ioutil.ReadFile("/sys/class/thermal/thermal_zone0/temp")
		if err != nil {
			logger.Warningf("reading /sys/class/thermal/thermal_zone0/temp failed: %v", err)
			return
		}
		tt, err := strconv.Atoi(strings.TrimSpace(string(t)))
		if err != nil {
			logger.Warningf("could not parse temp output: %v, err: %v", tt, err)
			return
		}
		temp = float64(tt) / 1000
	}

	// based on https://github.com/capnm/sysinfo/blob/master/sysinfo.go#L37
	var uptime time.Duration
	var load1 float64
	var load5 float64
	var load15 float64
	var procCount uint64
	var freeRamPerc float64
	var freeSwapPerc float64
	{
		si := syscall.Sysinfo_t{}
		err := syscall.Sysinfo(&si)
		if err != nil {
			logger.Warningf("sysinfo failed: %v", err)
			return
		}

		uptime = time.Duration(si.Uptime) * time.Second

		scale := 65536.0 // magic
		load1 = float64(si.Loads[0]) / scale
		load5 = float64(si.Loads[1]) / scale
		load15 = float64(si.Loads[2]) / scale
		procCount = uint64(si.Procs)

		unit := uint64(si.Unit) * 1024 // kB
		totalRam := uint64(si.Totalram) / unit
		totalSwap := uint64(si.Totalswap) / unit
		freeRam := uint64(si.Freeram) / unit
		freeSwap := uint64(si.Freeswap) / unit

		freeRamPerc = (float64(freeRam) / float64(totalRam)) * 100
		freeSwapPerc = (float64(freeSwap) / float64(totalSwap)) * 100
	}

	// from https://gist.github.com/lunny/9828326
	var freeRootPerc float64
	{
		fs := syscall.Statfs_t{}
		err := syscall.Statfs("/", &fs)
		if err != nil {
			logger.Warningf("statfs failed: %v", err)
			return
		}
		total := fs.Blocks * uint64(fs.Bsize)
		totalFree := fs.Bfree * uint64(fs.Bsize)
		free := total - totalFree
		freeRootPerc = (float64(free) / float64(total)) * 100
	}

	msg := fmt.Sprintf(
		"[%.1f°C | CPU: %.1f %.1f %.1f | Proc: %v | Free: %.1f%%(ram) %.1f%%(swap) %.1f%%(/) | Up: %v]",
		temp,
		load1,
		load5,
		load15,
		procCount,
		freeRamPerc,
		freeSwapPerc,
		freeRootPerc,
		uptime,
	)
	err := s.bot.Send(msg, true)
	if err != nil {
		logger.Warningf("sending message failed: %v", err)
	}
}

func (s *Status) CheckFile(path string) {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Tracef("file did not exist, skipping")
		} else {
			logger.Warningf("could not open file: %v", err)
		}
		return
	}
	defer f.Close()

	// make sure the file isnt currently being written
	stat, err := f.Stat()
	if err != nil {
		logger.Warningf("could not stat file: %v", err)
		return
	}
	if diff := time.Now().Sub(stat.ModTime()); diff > 0 && diff < time.Second {
		logger.Tracef("file %v modified in the last second, waiting", path)
		return
	}

	d, err := ioutil.ReadAll(f)
	if err != nil {
		logger.Warningf("could not readall file: %v", err)
		return
	}

	filename := time.Now().Format("20060102_150405_") + filepath.Base(path) + ".txt"
	buf, err := s.zip(d, filename)
	if err != nil {
		logger.Warningf("zipping file failed: %v", err)
		return
	}
	err = s.bot.SendFile(buf.Bytes(), filename+".zip", true)
	if err != nil {
		logger.Warningf("sending file failed: %v", err)
		return
	}

	// truncate file
	err = f.Truncate(0)
	if err != nil {
		logger.Warningf("failed to truncate file: %v", err)
	}

	_ = f.Sync()
}

func (s *Status) zip(in []byte, filename string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	f, err := w.Create(filename)
	if err != nil {
		return nil, err
	}
	_, err = f.Write(in)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return buf, nil
}
