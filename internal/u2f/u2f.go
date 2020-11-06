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
	"github.com/gsora/fidati"
	"github.com/gsora/fidati/keyring"
	"github.com/gsora/fidati/u2fhid"
	"github.com/gsora/fidati/u2ftoken"

	"github.com/f-secure-foundry/tamago/soc/imx6"
	"github.com/f-secure-foundry/tamago/soc/imx6/usb"
)

// Present is a channel used to signal user presence.
var Presence chan bool

var u2fKeyring keyring.Keyring

func Configure(device *usb.Device, u2fPublicKey []byte, u2fPrivateKey []byte) (err error) {
	token, err := u2ftoken.New(&u2fKeyring, u2fPublicKey, u2fPrivateKey)

	if err != nil {
		return
	}

	hid, err := u2fhid.NewHandler(token)

	if err != nil {
		return
	}

	if err != nil {
		return
	}

	err = fidati.ConfigureUSB(device.Configurations[0], device, hid)

	if err != nil {
		return
	}

	numInterfaces := len(device.Configurations[0].Interfaces)

	// resolve conflict with Ethernet over USB
	device.Configurations[0].Interfaces[numInterfaces-1].Endpoints[usb.OUT].EndpointAddress = 0x04
	device.Configurations[0].Interfaces[numInterfaces-1].Endpoints[usb.IN].EndpointAddress = 0x84

	return
}

func Init(managed bool) (err error) {
	if managed {
		Presence = make(chan bool)
	}

	counter := &Counter{}
	err = counter.Init(Presence)

	if err != nil {
		return
	}

	key, err := imx6.DCP.DeriveKey([]byte(DiversifierU2F), make([]byte, 16), -1)

	if err != nil {
		return
	}

	u2fKeyring.MasterKey = key
	u2fKeyring.Counter = counter

	return
}