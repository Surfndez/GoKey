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
	"crypto/sha256"
	"errors"
	"fmt"
	"log"

	"golang.org/x/crypto/pbkdf2"

	"github.com/f-secure-foundry/GoKey/internal/snvs"

	"github.com/f-secure-foundry/tamago/soc/imx6"
	"github.com/f-secure-foundry/tamago/soc/imx6/usb"

	"github.com/gsora/fidati"
	"github.com/gsora/fidati/keyring"
	"github.com/gsora/fidati/u2fhid"
	"github.com/gsora/fidati/u2ftoken"
)

// Present is a channel used to signal user presence.
var Presence chan bool

var u2fKeyring *keyring.Keyring

func Configure(device *usb.Device, u2fPublicKey []byte, u2fPrivateKey []byte, SNVS bool) (err error) {
	k := &keyring.Keyring{}

	if SNVS && len(u2fPrivateKey) != 0 {
		u2fPrivateKey, err = snvs.Decrypt(u2fPrivateKey, []byte(DiversifierU2F))

		if err != nil {
			return fmt.Errorf("key decryption failed, %v", err)
		}
	}

	token, err := u2ftoken.New(k, u2fPublicKey, u2fPrivateKey)

	if err != nil {
		return
	}

	hid, err := u2fhid.NewHandler(token)

	if err != nil {
		return
	}

	err = fidati.ConfigureUSB(device.Configurations[0], device, hid)

	if err != nil {
		return
	}

	// resolve conflict with Ethernet over USB
	numInterfaces := len(device.Configurations[0].Interfaces)
	device.Configurations[0].Interfaces[numInterfaces-1].Endpoints[usb.OUT].EndpointAddress = 0x04
	device.Configurations[0].Interfaces[numInterfaces-1].Endpoints[usb.IN].EndpointAddress = 0x84

	u2fKeyring = k

	return
}

func Init(managed bool) (err error) {
	if u2fKeyring == nil {
		return errors.New("U2F token initialization failed, missing configuration")
	}

	if managed {
		Presence = make(chan bool)
	} else {
		Presence = nil
	}

	counter := &Counter{}
	info, cnt, err := counter.Init(Presence)

	if err != nil {
		return
	}

	var mk []byte

	if imx6.DCP.SNVS() {
		mk, err = imx6.DCP.DeriveKey([]byte(DiversifierU2F), make([]byte, 16), -1)

		if err != nil {
			return
		}
	} else {
		uid := imx6.UniqueID()
		mk = pbkdf2.Key([]byte(info), uid[:], 4096, 16, sha256.New)
	}

	u2fKeyring.MasterKey = mk
	u2fKeyring.Counter = counter

	log.Printf("U2F token initialized, managed:%v counter:%d", managed, cnt)

	return
}
