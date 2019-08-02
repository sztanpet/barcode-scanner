package wifi

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("main.wifi")

func Setup(ssid, pw string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// nmcli -t -c no --fields NAME con show --active
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-c", "no", "--fields", "NAME", "con", "show", "--active")
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Criticalf("error running: nmcli -t -c no --fields NAME con show --active; error was: %v, output was: %s", err, out)
		return err
	}
	logger.Debugf("nmcli -t -c no --fields NAME con show --active; output was: %s", out)

	// go line by line and delete the active connections
	buf := bytes.NewBuffer(out)
	sc := bufio.NewScanner(buf)
	for sc.Scan() {
		name := sc.Text()
		if strings.Contains(name, "Wired") {
			logger.Tracef("not deleting wired connection: %v", name)
			continue
		}

		logger.Debugf("deleting connection: %v", name)

		// nmcli con delete <name>
		cmd = exec.CommandContext(ctx, "nmcli", "con", "delete", name)
		out, err = cmd.CombinedOutput()
		if err != nil {
			logger.Criticalf("error running: nmcli con delete '%q', error was: %v, output was: %s", name, err, out)
			return err
		}
		logger.Debugf("nmcli con delete %q; output was: %s", name, out)
	}
	// any errors while scanning?
	if err := sc.Err(); err != nil {
		logger.Errorf("scanner returned error: %v", err)
		return err
	}

	// nmcli device wifi connect <ssid> password <pw>
	cmd = exec.CommandContext(ctx, "nmcli", "device", "wifi", "connect", ssid, "password", pw)
	out, err = cmd.CombinedOutput()
	if err != nil {
		logger.Criticalf("error running: nmcli device wifi connect %q password %q, error was: %v, output was: %s", ssid, pw, err, out)
		return err
	}
	logger.Debugf("nmcli device wifi connect %q password %q; output was: %s", ssid, pw, out)
	return nil
}
