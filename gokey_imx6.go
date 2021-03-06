// https://github.com/f-secure-foundry/GoKey
//
// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// +build tamago,arm

package main

import (
	"fmt"
	"log"
	"net"

	"github.com/f-secure-foundry/GoKey/internal"
	"github.com/f-secure-foundry/GoKey/internal/ccid"
	"github.com/f-secure-foundry/GoKey/internal/icc"
	"github.com/f-secure-foundry/GoKey/internal/u2f"
	"github.com/f-secure-foundry/GoKey/internal/usb"

	"github.com/f-secure-foundry/tamago/dma"
	"github.com/f-secure-foundry/tamago/soc/imx6"
	imxusb "github.com/f-secure-foundry/tamago/soc/imx6/usb"
	"github.com/f-secure-foundry/tamago/soc/imx6/usb/ethernet"

	_ "github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"
)

const (
	IP        = "10.0.0.10"
	deviceMAC = "1a:55:89:a2:69:41"
	hostMAC   = "1a:55:89:a2:69:42"
)

func init() {
	if err := imx6.SetARMFreq(900); err != nil {
		panic(fmt.Sprintf("WARNING: error setting ARM frequency: %v\n", err))
	}
}

func main() {
	card := &icc.Interface{}
	token := &u2f.Token{}

	log.Println(gokey.Banner)

	device := &imxusb.Device{}
	usb.ConfigureDevice(device)

	if len(pgpSecretKey) != 0 {
		// Initialize an OpenPGP card with the bundled key information (defined
		// in `keys.go` and generated at compilation time).
		card.SNVS = SNVS
		card.ArmoredKey = pgpSecretKey
		card.Name = NAME
		card.Language = LANGUAGE
		card.Sex = SEX
		card.URL = URL
		card.Debug = false

		if initAtBoot {
			err := card.Init()

			if err != nil {
				log.Printf("OpenPGP ICC initialization error: %v", err)
			}
		}

		// initialize CCID interface
		reader := &ccid.Interface{
			ICC: card,
		}

		// set card serial number to 2nd half of NXP Unique ID
		uid := imx6.UniqueID()
		copy(card.Serial[0:4], uid[4:8])

		// configure Smart Card over USB endpoints (CCID protocol)
		usb.ConfigureCCID(device, reader)
	}

	if len(u2fPublicKey) != 0 && len(u2fPrivateKey) != 0 {
		token.SNVS = SNVS
		token.PublicKey = u2fPublicKey
		token.PrivateKey = u2fPrivateKey

		err := u2f.Configure(device, token)

		if err != nil {
			log.Printf("U2F configuration error: %v", err)
		}

		if initAtBoot {
			err = token.Init()

			if err != nil {
				log.Printf("U2F initialization error: %v", err)
			}
		}
	}

	if len(sshPublicKey) != 0 {
		configureNetworking(device, card, token)
	}

	dma.Init(dmaStart, dmaSize)

	imxusb.USB1.Init()
	imxusb.USB1.DeviceMode()
	imxusb.USB1.Reset()

	if err := imx6.SetARMFreq(198); err != nil {
		log.Fatalf("WARNING: error setting ARM frequency: %v\n", err)
	}

	// never returns
	imxusb.USB1.Start(device)
}

func configureNetworking(device *imxusb.Device, card *icc.Interface, token *u2f.Token) {
	// start basic networking
	stack, link := gokey.StartNetworking(deviceMAC, IP)

	if len(sshPublicKey) != 0 {
		console := &gokey.Console{
			Stack:         stack,
			Interface:     1,
			Address:       IP,
			Port:          22,
			AuthorizedKey: sshPublicKey,
			PrivateKey:    sshPrivateKey,
			Card:          card,
			Token:         token,
			Started:       make(chan bool),
		}

		// start SSH server for management console
		err := console.Start()

		if err != nil {
			log.Printf("SSH server initialization error: %v", err)
		}

		// wait for ssh server to start before responding to USB requests
		<-console.Started
	}

	hostAddress, err := net.ParseMAC(hostMAC)

	if err != nil {
		log.Fatal(err)
	}

	deviceAddress, err := net.ParseMAC(deviceMAC)

	if err != nil {
		log.Fatal(err)
	}

	// Configure Ethernet over USB endpoints
	// (ECM protocol, only supported on Linux hosts).
	eth := ethernet.NIC{
		Host:   hostAddress,
		Device: deviceAddress,
		Link:   link,
	}

	err = eth.Init(device, 0)

	if err != nil {
		log.Fatal(err)
	}
}
