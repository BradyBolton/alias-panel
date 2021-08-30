package main

import (
	"flag"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/BradyBolton/alias-panel/panel"
	"github.com/BradyBolton/alias-panel/parser"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

// Flag pointers
var dfp = flag.Bool("debug", false, "Log debug statements in aliaspanel.log")
var mfp = flag.String("margin", "2", "Margin size (default 2)")

func init() {
	flag.Parse()
	if *dfp {
		log.SetLevel(log.DebugLevel)
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
	} else {
		log.SetOutput(ioutil.Discard)
	}
}

func main() {
	sm := parser.ParseAll()
	log.Debugf("Sections: \n%v", spew.Sdump(sm))
	if m, err := strconv.Atoi(*mfp); err == nil {
		panel.DrawScreen(sm, m)
	} else {
		panic(err)
	}
}
