package router

import (
	"sync"
	"time"

	"github.com/uhppoted/uhppoted-tunnel/log"
)

type Switch struct {
	relay func(uint32, []byte)
}

type Router struct {
	handlers map[uint32]handler
	idletime time.Duration
	closed   chan bool
	sync.RWMutex
}

type handler struct {
	f       func([]byte)
	touched time.Time
}

var router = Router{
	handlers: map[uint32]handler{},
	idletime: 15 * time.Second,
	closed:   make(chan bool),
}

var ticker = time.NewTicker(15 * time.Second)

func init() {
	go func() {
		for {
			select {
			case <-router.closed:
				return

			case <-ticker.C:
				router.Sweep()
			}
		}
	}()
}

func NewSwitch(f func(uint32, []byte)) Switch {
	return Switch{
		relay: f,
	}
}

func (s *Switch) Request(id uint32, message []byte, h func([]byte)) {
	router.Add(id, h)

	go func() {
		s.relay(id, message)
	}()
}

func (s *Switch) Reply(id uint32, message []byte) {
	if hf := router.Get(id); hf != nil {
		go func() {
			hf(message)
		}()
	}
}

func (r *Router) Add(id uint32, h func([]byte)) {
	router.Lock()
	defer router.Unlock()

	router.handlers[id] = handler{
		f:       h,
		touched: time.Now(),
	}
}

func (r *Router) Get(id uint32) func([]byte) {
	if h, ok := r.handlers[id]; ok && h.f != nil {
		h.touched = time.Now()
		return h.f
	}

	return nil
}

func (r *Router) Sweep() {
	r.Lock()
	defer r.Unlock()

	cutoff := time.Now().Add(-r.idletime)
	idle := []uint32{}

	for k, v := range r.handlers {
		if v.touched.Before(cutoff) {
			idle = append(idle, k)
		}
	}

	for _, k := range idle {
		debugf("removing idle handler function (%v)", k)
		delete(r.handlers, k)
	}
}

func Close() {
	router.closed <- true
}

func debugf(format string, args ...any) {
	log.Debugf(format, args...)
}
