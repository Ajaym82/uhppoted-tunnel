package tunnel

import (
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/uhppoted/uhppoted-tunnel/log"
	"github.com/uhppoted/uhppoted-tunnel/router"
)

type UDP interface {
	Close()
	Run(*router.Switch) error
	Send(uint32, []byte)
}

type TCP interface {
	Close()
	Run(*router.Switch) error
	Send(uint32, []byte)
}

type Tunnel struct {
	udp UDP
	tcp TCP
}

func NewTunnel(udp UDP, tcp TCP) *Tunnel {
	return &Tunnel{
		udp: udp,
		tcp: tcp,
	}
}

func (t *Tunnel) Run(interrupt chan os.Signal) {
	infof("", "%v", "uhppoted-tunnel::run")

	q := router.NewSwitch(func(id uint32, message []byte) {
		t.udp.Send(id, message)
	})

	p := router.NewSwitch(func(id uint32, message []byte) {
		t.tcp.Send(id, message)
	})

	go func() {
		if err := t.udp.Run(&p); err != nil {
			fatalf("%v", err)
		}
	}()

	go func() {
		if err := t.tcp.Run(&q); err != nil {
			fatalf("%v", err)
		}
	}()

	<-interrupt

	infof("", "closing")

	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		router.Close()
	}()

	go func() {
		defer wg.Done()
		t.udp.Close()
	}()

	go func() {
		defer wg.Done()
		t.tcp.Close()
	}()

	wg.Wait()
	infof("", "closed")
}

func dump(m []byte, prefix string) string {
	regex := regexp.MustCompile("(?m)^(.*)")

	return fmt.Sprintf("%s", regex.ReplaceAllString(hex.Dump(m), prefix+"$1"))
}

func dumpf(message []byte, format string, args ...any) {
	hex := dump(message, "                                ")
	preamble := fmt.Sprintf(format, args...)

	debugf("UDP  %v\n%s", preamble, hex)
}

func debugf(format string, args ...any) {
	log.Debugf(format, args...)
}

func infof(tag string, format string, args ...any) {
	f := fmt.Sprintf("%-6v %v", tag, format)

	log.Infof(f, args...)
}

func warnf(tag, format string, args ...any) {
	f := fmt.Sprintf("%-6v %v", tag, format)

	log.Warnf(f, args...)
}

func errorf(tag string, format string, args ...any) {
	f := fmt.Sprintf("%-6v %v", tag, format)

	log.Errorf(f, args...)
}

func fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}
