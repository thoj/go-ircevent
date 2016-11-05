package irc

import (
	"crypto/tls"
	"math/rand"
	"testing"
	"time"
)

const server = "irc.freenode.net:6667"
const serverssl = "irc.freenode.net:7000"
const channel = "#go-eventirc-test"
const dict = "abcdefghijklmnopqrstuvwxyz"

//Spammy
const verbose_tests = false
const debug_tests = true

func TestConnectionEmtpyServer(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	err := irccon.Connect("")
	if err == nil {
		t.Fatal("emtpy server string not detected")
	}
}

func TestConnectionDoubleColon(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	err := irccon.Connect("::")
	if err == nil {
		t.Fatal("wrong number of ':' not detected")
	}
}

func TestConnectionMissingHost(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	err := irccon.Connect(":6667")
	if err == nil {
		t.Fatal("missing host not detected")
	}
}

func TestConnectionMissingPort(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	err := irccon.Connect("chat.freenode.net:")
	if err == nil {
		t.Fatal("missing port not detected")
	}
}

func TestConnectionNegativePort(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	err := irccon.Connect("chat.freenode.net:-1")
	if err == nil {
		t.Fatal("negative port number not detected")
	}
}

func TestConnectionTooLargePort(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	err := irccon.Connect("chat.freenode.net:65536")
	if err == nil {
		t.Fatal("too large port number not detected")
	}
}

func TestConnectionMissingLog(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.Log = nil
	err := irccon.Connect("chat.freenode.net:6667")
	if err == nil {
		t.Fatal("missing 'Log' not detected")
	}
}

func TestConnectionEmptyUser(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	// user may be changed after creation
	irccon.user = ""
	err := irccon.Connect("chat.freenode.net:6667")
	if err == nil {
		t.Fatal("empty 'user' not detected")
	}
}

func TestConnectionEmptyNick(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	// nick may be changed after creation
	irccon.nick = ""
	err := irccon.Connect("chat.freenode.net:6667")
	if err == nil {
		t.Fatal("empty 'nick' not detected")
	}
}

func TestRemoveCallback(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	debugTest(irccon)

	done := make(chan int, 10)

	irccon.AddCallback("TEST", func(e *Event) { done <- 1 })
	id := irccon.AddCallback("TEST", func(e *Event) { done <- 2 })
	irccon.AddCallback("TEST", func(e *Event) { done <- 3 })

	// Should remove callback at index 1
	irccon.RemoveCallback("TEST", id)

	irccon.RunCallbacks(&Event{
		Code: "TEST",
	})

	var results []int

	results = append(results, <-done)
	results = append(results, <-done)

	if len(results) != 2 || results[0] == 2 || results[1] == 2 {
		t.Error("Callback 2 not removed")
	}
}

func TestWildcardCallback(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	debugTest(irccon)

	done := make(chan int, 10)

	irccon.AddCallback("TEST", func(e *Event) { done <- 1 })
	irccon.AddCallback("*", func(e *Event) { done <- 2 })

	irccon.RunCallbacks(&Event{
		Code: "TEST",
	})

	var results []int

	results = append(results, <-done)
	results = append(results, <-done)

	if len(results) != 2 || !(results[0] == 1 && results[1] == 2) {
		t.Error("Wildcard callback not called")
	}
}

func TestClearCallback(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	debugTest(irccon)

	done := make(chan int, 10)

	irccon.AddCallback("TEST", func(e *Event) { done <- 0 })
	irccon.AddCallback("TEST", func(e *Event) { done <- 1 })
	irccon.ClearCallback("TEST")
	irccon.AddCallback("TEST", func(e *Event) { done <- 2 })
	irccon.AddCallback("TEST", func(e *Event) { done <- 3 })

	irccon.RunCallbacks(&Event{
		Code: "TEST",
	})

	var results []int

	results = append(results, <-done)
	results = append(results, <-done)

	if len(results) != 2 || !(results[0] == 2 && results[1] == 3) {
		t.Error("Callbacks not cleared")
	}
}

func TestIRCemptyNick(t *testing.T) {
	irccon := IRC("", "go-eventirc")
	irccon = nil
	if irccon != nil {
		t.Error("empty nick didn't result in error")
		t.Fail()
	}
}

func TestIRCemptyUser(t *testing.T) {
	irccon := IRC("go-eventirc", "")
	if irccon != nil {
		t.Error("empty user didn't result in error")
	}
}
func TestConnection(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ircnick1 := randStr(8)
	ircnick2 := randStr(8)
	irccon1 := IRC(ircnick1, "IRCTest1")

	irccon1.PingFreq = time.Second * 3

	debugTest(irccon1)

	irccon2 := IRC(ircnick2, "IRCTest2")
	debugTest(irccon2)

	teststr := randStr(20)
	testmsgok := make(chan bool, 1)

	irccon1.AddCallback("001", func(e *Event) { irccon1.Join(channel) })
	irccon2.AddCallback("001", func(e *Event) { irccon2.Join(channel) })
	irccon1.AddCallback("366", func(e *Event) {
		go func(e *Event) {
			tick := time.NewTicker(1 * time.Second)
			i := 10
			for {
				select {
				case <-tick.C:
					irccon1.Privmsgf(channel, "%s\n", teststr)
					if i == 0 {
						t.Errorf("Timeout while wating for test message from the other thread.")
						return
					}

				case <-testmsgok:
					tick.Stop()
					irccon1.Quit()
					return
				}
				i -= 1
			}
		}(e)
	})

	irccon2.AddCallback("366", func(e *Event) {
		ircnick2 = randStr(8)
		irccon2.Nick(ircnick2)
	})

	irccon2.AddCallback("PRIVMSG", func(e *Event) {
		if e.Message() == teststr {
			if e.Nick == ircnick1 {
				testmsgok <- true
				irccon2.Quit()
			} else {
				t.Errorf("Test message came from an unexpected nickname")
			}
		} else {
			//this may fail if there are other incoming messages, unlikely.
			t.Errorf("Test message mismatch")
		}
	})

	irccon2.AddCallback("NICK", func(e *Event) {
		if irccon2.nickcurrent == ircnick2 {
			t.Errorf("Nick change did not work!")
		}
	})

	err := irccon1.Connect(server)
	if err != nil {
		t.Log(err.Error())
		t.Errorf("Can't connect to freenode.")
	}
	err = irccon2.Connect(server)
	if err != nil {
		t.Log(err.Error())
		t.Errorf("Can't connect to freenode.")
	}

	go irccon2.Loop()
	irccon1.Loop()
}

func TestReconnect(t *testing.T) {
	ircnick1 := randStr(8)
	irccon := IRC(ircnick1, "IRCTestRe")
	debugTest(irccon)

	connects := 0
	irccon.AddCallback("001", func(e *Event) { irccon.Join(channel) })

	irccon.AddCallback("366", func(e *Event) {
		connects += 1
		if connects > 2 {
			irccon.Privmsgf(channel, "Connection nr %d (test done)\n", connects)
			irccon.Quit()
		} else {
			irccon.Privmsgf(channel, "Connection nr %d\n", connects)
			time.Sleep(100) //Need to let the thraed actually send before closing socket
			irccon.Disconnect()
		}
	})

	err := irccon.Connect(server)
	if err != nil {
		t.Log(err.Error())
		t.Errorf("Can't connect to freenode.")
	}

	irccon.Loop()
	if connects != 3 {
		t.Errorf("Reconnect test failed. Connects = %d", connects)
	}
}

func TestConnectionSSL(t *testing.T) {
	ircnick1 := randStr(8)
	irccon := IRC(ircnick1, "IRCTestSSL")
	debugTest(irccon)
	irccon.UseTLS = true
	irccon.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	irccon.AddCallback("001", func(e *Event) { irccon.Join(channel) })

	irccon.AddCallback("366", func(e *Event) {
		irccon.Privmsg(channel, "Test Message from SSL\n")
		irccon.Quit()
	})

	err := irccon.Connect(serverssl)
	if err != nil {
		t.Log(err.Error())
		t.Errorf("Can't connect to freenode.")
	}

	irccon.Loop()
}

// Helper Functions
func randStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = dict[rand.Intn(len(dict))]
	}
	return string(b)
}

func debugTest(irccon *Connection) *Connection {
	irccon.VerboseCallbackHandler = verbose_tests
	irccon.Debug = debug_tests
	return irccon
}
