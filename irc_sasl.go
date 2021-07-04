package irc

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type SASLResult struct {
	Failed bool
	Err    error
}

// Check if a space-separated list of arguments contains a value.
func listContains(list string, value string) bool {
	for _, arg_name := range strings.Split(strings.TrimSpace(list), " ") {
		if arg_name == value {
			return true
		}
	}
	return false
}

func (irc *Connection) setupSASLCallbacks(result chan<- *SASLResult) (callbacks []CallbackID) {
	id := irc.AddCallback("CAP", func(e *Event) {
		if len(e.Arguments) == 3 {
			if e.Arguments[1] == "LS" {
				if !listContains(e.Arguments[2], "sasl") {
					result <- &SASLResult{true, errors.New("no SASL capability " + e.Arguments[2])}
				}
			}
			if e.Arguments[1] == "ACK" && listContains(e.Arguments[2], "sasl") {
				if irc.SASLMech != "PLAIN" && irc.SASLMech != "EXTERNAL" {
					result <- &SASLResult{true, errors.New("only PLAIN and EXTERNAL supported")}
				}
				irc.SendRaw("AUTHENTICATE " + irc.SASLMech)
			}
		}
	})
	callbacks = append(callbacks, CallbackID{"CAP", id})

	id = irc.AddCallback("AUTHENTICATE", func(e *Event) {
		if irc.SASLMech == "EXTERNAL" {
			irc.SendRaw("AUTHENTICATE +")
			return
		}
		str := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s\x00%s\x00%s", irc.SASLLogin, irc.SASLLogin, irc.SASLPassword)))
		irc.SendRaw("AUTHENTICATE " + str)
	})
	callbacks = append(callbacks, CallbackID{"AUTHENTICATE", id})

	id = irc.AddCallback("901", func(e *Event) {
		irc.SendRaw("CAP END")
		irc.SendRaw("QUIT")
		result <- &SASLResult{true, errors.New(e.Arguments[1])}
	})
	callbacks = append(callbacks, CallbackID{"901", id})

	id = irc.AddCallback("902", func(e *Event) {
		irc.SendRaw("CAP END")
		irc.SendRaw("QUIT")
		result <- &SASLResult{true, errors.New(e.Arguments[1])}
	})
	callbacks = append(callbacks, CallbackID{"902", id})

	id = irc.AddCallback("903", func(e *Event) {
		result <- &SASLResult{false, nil}
	})
	callbacks = append(callbacks, CallbackID{"903", id})

	id = irc.AddCallback("904", func(e *Event) {
		irc.SendRaw("CAP END")
		irc.SendRaw("QUIT")
		result <- &SASLResult{true, errors.New(e.Arguments[1])}
	})
	callbacks = append(callbacks, CallbackID{"904", id})

	return
}
