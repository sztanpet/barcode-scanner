// status implements monitoring of system status
// checks for new dmesg output and logs it
// checks for any error output from binaries
// checks for system temperature, uptime, and load
// dmesg and error output is zipped and sent via telegram
package status

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

func (s *Status) Check(filepath string) {
	s.dmesg()
	s.sysinfo()
	s.file(filepath)
}

func (s *Status) dmesg() {
	//cmd := exec.CommandContext(s.ctx, "dmesg", "-e", "-c")
	cmd := exec.CommandContext(s.ctx, "dmesg", "-e") // TODO delete
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("dmesg -e -c failed: %v", err)
		return
	}

	fmt.Printf("out was: %q\n", string(out))
	if len(out) == 0 {
		return
	}

	msg := time.Now().Format("2006-01-02 15:04:05") + " - dmesg output: "
	err = s.bot.SendFile(bytes.NewBuffer(out), msg, true)
	if err != nil {
		logger.Warningf("sending file failed: %v", err)
	}
}

func (s *Status) sysinfo() {
	// $ cat raspitemp.sh
	// cat /sys/class/thermal/thermal_zone0/temp
	// 43802 -> 43.802C

	// https://golang.org/pkg/syscall/#Sysinfo https://golang.org/pkg/syscall/#Sysinfo_t
	/*
		si := &unix.Sysinfo_t{}

		// XXX is a raw syscall thread safe?
		err := unix.Sysinfo(si)
		if err != nil {
			panic("Commander, we have a problem. syscall.Sysinfo:" + err.Error())
		}
		scale := 65536.0 // magic
		unit := uint64(si.Unit) * 1024 // kB

		sis.Uptime = time.Duration(si.Uptime) * time.Second
		sis.Loads[0] = float64(si.Loads[0]) / scale
		sis.Loads[1] = float64(si.Loads[1]) / scale
		sis.Loads[2] = float64(si.Loads[2]) / scale
		sis.Procs = uint64(si.Procs)

		sis.TotalRam = uint64(si.Totalram) / unit
		sis.FreeRam = uint64(si.Freeram) / unit
		sis.BufferRam = uint64(si.Bufferram) / unit
		sis.TotalSwap = uint64(si.Totalswap) / unit
		sis.FreeSwap = uint64(si.Freeswap) / unit
		sis.TotalHighRam = uint64(si.Totalhigh) / unit
		sis.FreeHighRam = uint64(si.Freehigh) / unit
	*/
}

func (s *Status) file(filepath string) {
	// TODO StandardError=file:/var/log2.log
	// check size, read and truncate if non-0
	// bot.Sendfile
	_ = filepath
}
