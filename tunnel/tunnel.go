package tunnel

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sync"

	"github.com/uhppoted/uhppoted-tunnel/log"
	"github.com/uhppoted/uhppoted-tunnel/router"
)

type Conn interface {
	Close()
	Run(*router.Switch) error
	Send(uint32, []byte)
}

type Message struct {
	id      uint32
	message []byte
}

type Tunnel struct {
	in  Conn
	out Conn
	ctx context.Context
}

func NewTunnel(in Conn, out Conn, ctx context.Context) *Tunnel {
	return &Tunnel{
		in:  in,
		out: out,
		ctx: ctx,
	}
}

func (t *Tunnel) Run(interrupt chan os.Signal) (err error) {
	infof("", "%v", "uhppoted-tunnel::run")

	p := router.NewSwitch(func(id uint32, message []byte) {
		t.out.Send(id, message)
	})

	q := router.NewSwitch(func(id uint32, message []byte) {
		t.in.Send(id, message)
	})

	ctx, cancel := context.WithCancel(t.ctx)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				fatalf("%v", err)
			}
		}()

		if err = t.in.Run(&p); err != nil {
			errorf("OUT", "%v", err)
			cancel()
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				fatalf("%v", err)
			}
		}()

		if err = t.out.Run(&q); err != nil {
			errorf("IN", "%v", err)
			cancel()
		}
	}()

	select {
	case <-t.ctx.Done():
	case <-ctx.Done():
	}

	infof("", "closing")

	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		router.Close()
	}()

	go func() {
		defer wg.Done()
		t.in.Close()
	}()

	go func() {
		defer wg.Done()
		t.out.Close()
	}()

	wg.Wait()
	infof("", "closed")

	return
}

func dump(m []byte, prefix string) string {
	regex := regexp.MustCompile("(?m)^(.*)")

	return fmt.Sprintf("%s", regex.ReplaceAllString(hex.Dump(m), prefix+"$1"))
}

func dumpf(tag string, message []byte, format string, args ...any) {
	hex := dump(message, "                                  ")
	preamble := fmt.Sprintf(format, args...)

	debugf(tag, "%v\n%s", preamble, hex)
}

func debugf(tag, format string, args ...any) {
	f := fmt.Sprintf("%-6v %v", tag, format)

	log.Debugf(f, args...)
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
	f := fmt.Sprintf("%-6v %v", "", format)

	log.Fatalf(f, args...)
}
