package wifi

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("main.wifi")

type Account struct {
	SSID, PW string
}

func StoreAndTry(ctx context.Context, cfg *config.Config, acc Account) error {
	if err := storeAccount(cfg, acc); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := connect(ctx, acc); err != nil {
		return err
	}

	return nil
}

func accountPath(cfg *config.Config) string {
	return filepath.Join(cfg.StatePath, "WiFiAccount")
}

func storeAccount(cfg *config.Config, acc Account) error {
	account, err := LoadAccount(cfg)
	if err != nil {
		return err
	}

	// does the account already exist?
	if account.SSID == acc.SSID && account.PW == acc.PW {
		return nil
	}

	logger.Debugf("storing account: %v", acc)
	if err := file.Serialize(accountPath(cfg), &acc); err != nil {
		return err
	}

	return nil
}

func LoadAccount(cfg *config.Config) (ret Account, err error) {
	p := accountPath(cfg)
	if !file.Exists(p) {
		logger.Debugf("accountPath did not exist, returning zero values")
		return
	}

	err = file.Unserialize(accountPath(cfg), &ret)
	logger.Debugf("loaded accounts: %#v error was: %v", ret, err)
	return
}

func deleteConnections(ctx context.Context) error {
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

	return nil
}

func connect(ctx context.Context, acc Account) error {
	if err := deleteConnections(ctx); err != nil {
		return err
	}

	// nmcli device wifi connect <SSID> password <PW>
	cmd := exec.CommandContext(ctx, "nmcli", "device", "wifi", "connect", acc.SSID, "password", acc.PW)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Criticalf("error running: nmcli device wifi connect %q password %q, error was: %v, output was: %s", acc.SSID, acc.PW, err, out)
		return err
	}

	logger.Debugf("nmcli command output was: %s", out)
	return nil
}

func IsConnected() bool {
	_, err := http.Get("http://clients3.google.com/generate_204")
	return err == nil
}

func Setup(ctx context.Context, cfg *config.Config) error {
	account, err := LoadAccount(cfg)
	if err != nil {
		return err
	}

	err = connect(ctx, account)
	logger.Debugf("wifi connect error for %v: %v", account, err)
	return err
}
