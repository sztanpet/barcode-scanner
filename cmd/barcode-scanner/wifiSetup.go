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
	a.screen.WriteHelp("Enter SSID name and press enter")
}
func (a *app) enterWifiSetupPW() {
	a.state = wifiSetupPW
	a.currentLine.Reset()

	a.screen.WriteLine(1, "Password:")
	a.screen.WriteLine(2, "")
	a.screen.WriteHelp("Enter PW and press enter")
}
func (a *app) enterWifiSetupDone() {
	a.state = wifiSetupDone
	a.currentLine.Reset()

	a.screen.WriteLine(1, "Checking…")
	a.screen.WriteLine(2, "")
	a.screen.WriteHelp("Please wait… (ESC to cancel)")
}

// handleWifiSetupInput is only called by transitionState
func (a *app) handleWifiSetupInput(r rune) {
	switch r {
	case '\r':
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

// cancelWifiSetup is only called by transitionState
func (a *app) cancelWifiSetup() {
	a.state = readBarcode
	wifiInfo.ssid = ""
	wifiInfo.pw = ""
	a.currentLine.Reset()
	a.enterReadBarcode()
}

// doneWifiSetup is called by handleWifiSetupInput (NOT transitionState)
func (a *app) doneWifiSetup() {
	logger.Debugf("State: wifiSetup -> readBarcode (doneWifiSetup)")
	a.cancelWifiSetup()
}
