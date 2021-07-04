package irc

import (
	"crypto/tls"
	"os"
	"testing"
	"time"
)

// set SASLLogin and SASLPassword environment variables before testing
func TestConnectionSASL(t *testing.T) {
	SASLServer := "irc.freenode.net:7000"
	SASLLogin := os.Getenv("SASLLogin")
	SASLPassword := os.Getenv("SASLPassword")

	if SASLLogin == "" {
		t.Skip("Define SASLLogin and SASLPasword environment varables to test SASL")
	}
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	irccon.Debug = true
	irccon.UseTLS = true
	irccon.UseSASL = true
	irccon.SASLLogin = SASLLogin
	irccon.SASLPassword = SASLPassword
	irccon.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366", func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message SASL\n")
		time.Sleep(2 * time.Second)
		irccon.Quit()
	})

	err := irccon.Connect(SASLServer)
	if err != nil {
		t.Fatalf("SASL failed: %s", err)
	}
	irccon.Loop()
}


// 1. Register fingerprint with IRC network
// 2. Add SASLKeyPem="-----BEGIN PRIVATE KEY-----..."
//    and SASLCertPem="-----BEGIN CERTIFICATE-----..."
//    to CI environment as masked variables
func TestConnectionSASLExternal(t *testing.T) {
	SASLServer := "irc.freenode.net:7000"
	keyPem := os.Getenv("SASLKeyPem")
	certPem := os.Getenv("SASLCertPem")

	if certPem == "" || keyPem == "" {
		t.Skip("Env vars SASLKeyPem SASLCertPem not present, skipping")
	}
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	cert, err := tls.X509KeyPair([]byte(certPem), []byte(keyPem))
	if err != nil {
		t.Fatalf("SASL EXTERNAL cert creation failed: %s", err)
	}

	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	irccon.Debug = true
	irccon.UseTLS = true
	irccon.UseSASL = true
	irccon.SASLMech = "EXTERNAL"
	irccon.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		Certificates: []tls.Certificate{cert},
	}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366", func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message SASL EXTERNAL\n")
		time.Sleep(2 * time.Second)
		irccon.Quit()
	})

	err = irccon.Connect(SASLServer)
	if err != nil {
		t.Fatalf("SASL EXTERNAL failed: %s", err)
	}
	irccon.Loop()
}
