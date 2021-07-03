package main

import (
	"github.com/thoj/go-ircevent"
	"crypto/tls"
	"log"
	"os"
)

const addr = "libera75jm6of4wxpxt4aynol3xjmbtxgfyjpu34ss4d7r7q2v5zrpyd.onion:6697"

// This demos connecting to Libera.Chat over TOR using SASL EXTERNAL and a TLS
// client cert. It assumes a TOR SOCKS service is running on localhost:9050
// and requires an existing account with a fingerprint already registered. See
// https://libera.chat/guides/connect#accessing-liberachat-via-tor for details.
//
// Pass the full path to your cert and key on the command line like so:
// $ go run simple-tor.go my-nick my-cert.pem my-key.pem

func main() {
	os.Setenv("ALL_PROXY", "socks5h://localhost:9050")
	nick, certFile := os.Args[1], os.Args[2]
	keyFile := certFile
	if len(os.Args) == 4 {
		keyFile = os.Args[3]
	}
	clientCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}
	ircnick1 := nick
	irccon := irc.IRC(ircnick1, nick)
	irccon.VerboseCallbackHandler = true
	irccon.UseSASL = true
	irccon.SASLMech = "EXTERNAL"
	irccon.Debug = true
	irccon.UseTLS = true
	irccon.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		Certificates: []tls.Certificate{clientCert},
	}
	irccon.AddCallback("001", func(e *irc.Event) {})
	irccon.AddCallback("376", func(e *irc.Event) {
		log.Println("Quitting")
		irccon.Quit()
	})
	err = irccon.Connect(addr)
	if err != nil {
		log.Fatal(err)
	}
	irccon.Loop()
}
