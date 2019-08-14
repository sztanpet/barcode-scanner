package main

import (
	"time"
	"unicode"
	"unicode/utf8"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/wifi"
)

var wifiInfo struct {
	ssid, pw string
} = struct {
	ssid, pw string
}{}

// enterWifiSetup is only called by transitionState
func (a *app) enterWifiSetup() {
	a.state = wifiSetupSSID
	wifiInfo.ssid = ""
	wifiInfo.pw = ""
	a.currentLine.Reset()

	a.screen.Clear()
	a.screen.WriteTitle("WI-FI SETUP")
	a.screen.WriteLine(1, "SSID:")
	a.screen.WriteLine(2, "")
	a.screen.WriteHelp("(ESC to cancel)")
}
func (a *app) enterWifiSetupPW() {
	a.state = wifiSetupPW
	a.currentLine.Reset()

	a.screen.Clear()
	a.screen.WriteTitle("WI-FI SETUP")
	a.screen.WriteLine(1, "Password:")
	a.screen.WriteLine(2, "")
	a.screen.WriteHelp("(ESC to cancel)")
}
func (a *app) enterWifiSetupDone() {
	a.state = wifiSetupDone
	a.currentLine.Reset()

	a.screen.Clear()
	a.screen.WriteTitle("WI-FI SETUP")
	a.screen.WriteLine(1, "Checking…")
	a.screen.WriteLine(2, "Please wait…")
	a.screen.WriteHelp("")

	err := wifi.Setup(wifiInfo.ssid, wifiInfo.pw)
	if err != nil {
		a.screen.WriteLine(2, "Error!")
		logger.Criticalf("wifi setup error: %v", err)
	} else {
		a.screen.WriteLine(2, "Success!")
	}
	time.Sleep(2 * time.Second)
	a.doneWifiSetup()
}

// cancelWifiSetup is only called by transitionState
func (a *app) cancelWifiSetup() {
	a.state = readBarcode
	wifiInfo.ssid = ""
	wifiInfo.pw = ""
	a.currentLine.Reset()
	a.enterReadBarcode()
}

func (a *app) doneWifiSetup() {
	logger.Debugf("handleWifiSetupInput: doneWifiSetup -> readBarcode")
	a.cancelWifiSetup()
}

// handleWifiSetupInput is only called by transitionState
func (a *app) handleWifiSetupInput(r rune) {
	switch r {
	case '\n':
		line := a.currentLine.String()
		switch a.state {
		case wifiSetupSSID:
			wifiInfo.ssid = line
			if len(line) > 0 {
				a.enterWifiSetupPW()
			}

		case wifiSetupPW:
			wifiInfo.pw = line
			if len(line) > 0 {
				a.enterWifiSetupDone()
			}

		case wifiSetupDone:
			// nothing to do
		default:
			panic("unhandled state " + string(rune(a.state+'0')))
		}

	case tty.KeyEscape:
		logger.Debugf("handleWifiSetupInput: escape pressed; state was: %v", a.state)
		a.cancelWifiSetup()

	case tty.KeyBackspace, tty.KeyDelete:
		// https://stackoverflow.com/questions/39907667/how-to-remove-unicode-characters-from-byte-buffer-in-go
		if a.currentLine.Len() >= 1 {
			b := a.currentLine.Bytes()
			i := 0
			for i < len(b) {
				_, n := utf8.DecodeRune(b[i:])
				if i+n == len(b) {
					a.currentLine.Truncate(i)
					break
				}
				i += n
			}

			a.screen.WriteLine(2, a.currentLine.String())
			logger.Tracef("handleWifiSetupInput: backspace")
		}
	default:
		if unicode.IsPrint(r) {
			_, _ = a.currentLine.WriteRune(r)
			a.screen.WriteLine(2, a.currentLine.String())
		}
	}
}
