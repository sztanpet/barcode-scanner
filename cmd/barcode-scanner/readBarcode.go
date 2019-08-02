package main

import (
	"strings"
	"time"
	"unicode"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
)

func (a *app) handleReadBarcode(r rune) {
	switch r {
	case tty.KeyEscape:
		logger.Debugf("State: readBarcode -> wifiSetup (escape pressed)")
		a.enterWifiSetup()
		return

	case '\n':
		a.handleBarcodeDone()
	default:
		a.handleBarcodeInput(r)
	}
}

// enterReadBarcode is called by cancelWifiSetup and doneWifiSetup
func (a *app) enterReadBarcode() {
	a.state = readBarcode
	a.currentLine.Reset()

	// clear and init screen
	a.screen.Clear()
	a.screen.WriteTitle("SCANNER")
	a.screen.WriteLine(1, "Barcode data:")
	a.screen.WriteHelp("waiting for scan")
}

// handleBarcodeInput is only called by transitionState
// it appends the new rune to a.currentLine and displays it on the screen
func (a *app) handleBarcodeInput(r rune) {
	if r > unicode.MaxASCII || !unicode.IsPrint(r) {
		logger.Debugf("handleBarcodeInput: got invalid input: %x %q, ignoring", r, r)
		return
	}

	_, _ = a.currentLine.WriteRune(r)
}

// handleBarcodeDone signals that a new barcode is available in a.currentLine
func (a *app) handleBarcodeDone() {
	bc := a.currentLine.String()
	a.currentLine.Reset()

	line := strings.TrimSpace(bc)
	if len(line) == 0 {
		logger.Debugf("handleBarcodeInput: empty currentLine, skipping")

		go func() {
			if err := buzzer.FailBeep(); err != nil {
				logger.Infof("buzzer.FailBeep failed: %v", err)
			}
		}()
		return
	}

	a.screen.WriteLine(2, bc)
	if a.handleSpecialBarcode(bc) {
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	b := storage.Barcode{
		Barcode:        bc,
		Direction:      a.dir.String(),
		CurrierService: a.currier,
		CreatedAt:      time.Now(),
	}
	logger.Tracef("inserting barcode: %#v", b)
	a.storage.Insert(b)

	go func() {
		if err := buzzer.SuccessBeep(); err != nil {
			logger.Infof("buzzer.SuccessBeep failed: %v", err)
		}
	}()
}

func (a *app) handleSpecialBarcode(bc string) bool {
	matches := specialBarcodeRe.FindStringSubmatch(bc)
	if matches == nil {
		return false
	}
	logger.Tracef("special barcode matched: %v", matches)

	a.mu.Lock()
	defer a.mu.Unlock()

	switch strings.ToUpper(matches[1]) {
	case "EGRESS":
		a.dir = EGRESS
	case "INGRESS":
		a.dir = INGRESS
	default:
		panic("unexpected direction: " + matches[1])
	}

	a.currier = matches[2]

	a.persistSettingsLocked()
	return true
}
