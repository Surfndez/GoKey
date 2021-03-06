// https://github.com/f-secure-foundry/GoKey
//
// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// +build tamago,arm

package u2f

import (
	"encoding/binary"
	"log"
	"runtime"
	"time"

	"github.com/f-secure-foundry/armoryctl/atecc608"
	"github.com/f-secure-foundry/armoryctl/led"
)

const (
	counterCmd = 0x24
	read       = 0
	increment  = 1
	// Counter KeyID, #1 is used as it is never attached to any key.
	keyID = 0x01
	// user presence timeout in seconds
	timeout = 10
)

// Counter represents an ATECC608A based monotonic counter instance.
type Counter struct {
	presence chan bool
}

// Init initializes an ATECC608A backed U2F counter. A channel can be passed to
// receive user presence notifications, if nil user presence is automatically
// assumed.
func (c *Counter) Init(presence chan bool) (err error) {
	c.presence = presence
	 _, err = atecc608.SelfTest()
	return
}

// Info gathers the ATECC608A random S/N and model.
func (c *Counter) Info() (info string, err error) {
	return atecc608.Info()
}

// Increment increases the ATECC608A monotonic counter in slot <1> (not attached to any key).
func (c *Counter) Increment(_ []byte, _ []byte, _ []byte) (cnt uint32, err error) {
	cnt, err = c.cmd(increment)

	if err != nil {
		log.Printf("U2F increment failed, %v", err)
		return
	}

	log.Printf("U2F increment, counter:%d", cnt)

	return
}

// Read reads the ATECC608A monotonic counter in slot <1> (not attached to any key).
func (c *Counter) Read() (cnt uint32, err error) {
	return c.cmd(read)
}

// UserPresence verifies the user presence.
func (c *Counter) UserPresence() (present bool) {
	if c.presence == nil {
		return true
	}

	var done = make(chan bool)
	go blink(done)

	log.Printf("U2F user presence request, type `p` within %ds to confirm", timeout)

	select {
	case <-c.presence:
		present = true
	case <-time.After(timeout * time.Second):
		log.Printf("U2F user presence request timed out")
	}

	done <- true

	if present {
		log.Printf("U2F user presence confirmed")
	}

	return
}

func (c *Counter) cmd(mode byte) (cnt uint32, err error) {
	res, err := atecc608.ExecuteCmd(counterCmd, [1]byte{mode}, [2]byte{keyID, 0x00}, nil, true)

	if err != nil {
		return
	}

	return binary.LittleEndian.Uint32(res), nil
}

func blink(done chan bool) {
	var on bool

	for {
		select {
		case <-done:
			led.Set("white", false)
			return
		default:
		}

		on = !on
		led.Set("white", on)

		runtime.Gosched()
		time.Sleep(200 * time.Millisecond)
	}
}
