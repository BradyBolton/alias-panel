package main

import (
	"os"

	"github.com/BradyBolton/alias-panel/panel"
	"github.com/BradyBolton/alias-panel/parser"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.InfoLevel) // TODO: add a log level (optional argument)
	f, err := os.OpenFile("aliaspanel.log", os.O_CREATE|os.O_WRONLY, 0666)
	if err == nil {
		log.SetOutput(f)
	} else {
		log.Info("Failed to log to file, defaulting to stderr")
	}

	// See issue: https://github.com/sirupsen/logrus/issues/608
	log.SetFormatter(&log.TextFormatter{
		DisableQuote: true,
	})
}

func main() {
	sm := parser.ParseAll()
	log.Debugf("Sections: \n%v", spew.Sdump(sm))

	panel.DrawScreen(sm)
}
