package irc

import (
	//	"github.com/thoj/go-ircevent"
	"testing"
)

func TestConnection(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	err := irccon.Connect("irc.freenode.net:6667")
	if err != nil {
		t.Fatal("Can't connect to freenode.")
	}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366", func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message\n")
		irccon.Nick("go-eventnewnick")
	})
	irccon.AddCallback("NICK", func(e *Event) {
		irccon.Quit()
		if irccon.nickcurrent == "go-eventnewnick" {
			t.Fatal("Nick change did not work!")
		}
	})
	irccon.Loop()
}

func TestConnectionSSL(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	irccon.UseTLS = true
	err := irccon.Connect("irc.freenode.net:7000")
	if err != nil {
		t.Fatal("Can't connect to freenode.")
	}
	irccon.AddCallback("001", func(e *Event) { irccon.Join("#go-eventirc") })

	irccon.AddCallback("366", func(e *Event) {
		irccon.Privmsg("#go-eventirc", "Test Message\n")
		irccon.Quit()
	})

	irccon.Loop()
}

func TestRemoveCallback(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	
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

	if len(results) != 2 || !(results[0] == 1 && results[1] == 3) {
		t.Error("Callback 2 not removed")
	}
}

func TestWildcardCallback(t *testing.T) {
	irccon := IRC("go-eventirc", "go-eventirc")
	irccon.VerboseCallbackHandler = true
	
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
