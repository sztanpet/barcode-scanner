// +build !windows
// +build !plan9
// +build !amd64

// status implements monitoring of system status
// checks for new dmesg output and logs it
// checks for any error output from binaries
// checks for system temperature, uptime, and load
// dmesg and error output is zipped and sent via telegram
package status

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
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

type sysinfo struct {
	uptime       time.Duration
	load1        float64
	load5        float64
	load15       float64
	procCount    uint64
	freeRamPerc  float64
	freeSwapPerc float64
}

func New(ctx context.Context, bot *telegram.Bot) *Status {
	return &Status{
		ctx: ctx,
		bot: bot,
	}
}

func (s *Status) Check() {
	si := sysInfo()
	if si == nil {
		return
	}

	msg := fmt.Sprintf(
		"[%.1f°C | CPU: %.1f %.1f %.1f | Proc: %v | Free: %.1f%%(ram) %.1f%%(swap) %.1f%%(/) | Up: %v]",
		temp(),
		si.load1,
		si.load5,
		si.load15,
		si.procCount,
		si.freeRamPerc,
		si.freeSwapPerc,
		rootFSPercent(),
		si.uptime,
	)
	err := s.bot.Send(msg, true)
	if err != nil {
		logger.Warningf("sending message failed: %v", err)
	}
}

func temp() float64 {
	// $ cat raspitemp.sh
	// cat /sys/class/thermal/thermal_zone0/temp
	// 43802 -> 43.802C
	t, err := ioutil.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		logger.Warningf("reading /sys/class/thermal/thermal_zone0/temp failed: %v", err)
		return math.NaN()
	}
	tt, err := strconv.Atoi(strings.TrimSpace(string(t)))
	if err != nil {
		logger.Warningf("could not parse temp output: %v, err: %v", tt, err)
		return math.NaN()
	}

	return float64(tt) / 1000
}

func sysInfo() *sysinfo {
	// based on https://github.com/capnm/sysinfo/blob/master/sysinfo.go#L37
	ret := &sysinfo{}
	si := syscall.Sysinfo_t{}
	err := syscall.Sysinfo(&si)
	if err != nil {
		logger.Warningf("sysinfo failed: %v", err)
		return nil
	}

	ret.uptime = time.Duration(si.Uptime) * time.Second

	scale := 65536.0 // magic
	ret.load1 = float64(si.Loads[0]) / scale
	ret.load5 = float64(si.Loads[1]) / scale
	ret.load15 = float64(si.Loads[2]) / scale
	ret.procCount = uint64(si.Procs)

	unit := uint64(si.Unit) * 1024 // kB
	totalRam := uint64(si.Totalram) / unit
	totalSwap := uint64(si.Totalswap) / unit
	freeRam := uint64(si.Freeram) / unit
	freeSwap := uint64(si.Freeswap) / unit

	ret.freeRamPerc = (float64(freeRam) / float64(totalRam)) * 100
	ret.freeSwapPerc = (float64(freeSwap) / float64(totalSwap)) * 100

	return ret
}

func rootFSPercent() float64 {
	// from https://gist.github.com/lunny/9828326
	fs := syscall.Statfs_t{}
	err := syscall.Statfs("/", &fs)
	if err != nil {
		logger.Warningf("statfs failed: %v", err)
		return math.NaN()
	}

	total := fs.Blocks * uint64(fs.Bsize)
	totalFree := fs.Bfree * uint64(fs.Bsize)
	free := total - totalFree
	return (float64(free) / float64(total)) * 100
}
