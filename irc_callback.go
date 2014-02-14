package irc

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"time"
)


// Register a callback to a connection and event code. A callback is a function 
// which takes only an Event pointer as parameter. Valid event codes are: 
// 001 - welcoming on the IRC network
// 437 - if nickname or channel are currently not available
// 433 - if nickname is currently in use
// CTCP_CLIENTINFO
// CTCP_PING
// CTCP_TIME
// CTCP_USERINFO
// CTCP_VERSION
// PING - if PING is received
// PONG - if PONG is received
// NICK - if nickname (in connection) matches current message
func (irc *Connection) AddCallback(eventcode string, callback func(*Event)) string {
	eventcode = strings.ToUpper(eventcode)

	if _, ok := irc.events[eventcode]; !ok {
		irc.events[eventcode] = make(map[string]func(*Event))
	}
	h := sha1.New()
	rawId := []byte(fmt.Sprintf("%v%d", reflect.ValueOf(callback).Pointer(), rand.Int63()))
	h.Write(rawId)
	id := fmt.Sprintf("%x", h.Sum(nil))
	irc.events[eventcode][id] = callback
	return id
}

func (irc *Connection) RemoveCallback(eventcode string, i string) bool {
	eventcode = strings.ToUpper(eventcode)

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			delete(irc.events[eventcode], i)
			return true
		}
		irc.Log.Printf("Event found, but no callback found at id %s\n", i)
		return false
	}

	irc.Log.Println("Event not found")
	return false
}


// Remove all callbacks from a given event code. It returns true 
// if given event code is found and cleared.
func (irc *Connection) ClearCallback(eventcode string) bool {
	eventcode = strings.ToUpper(eventcode)

	if _, ok := irc.events[eventcode]; ok {
		irc.events[eventcode] = make(map[string]func(*Event))
		return true
	}

	irc.Log.Println("Event not found")
	return false
}

func (irc *Connection) ReplaceCallback(eventcode string, i string, callback func(*Event)) {
	eventcode = strings.ToUpper(eventcode)

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			event[i] = callback
			return
		}
		irc.Log.Printf("Event found, but no callback found at id %s\n", i)
	}
	irc.Log.Printf("Event not found. Use AddCallBack\n")
}


// Execute all callbacks associated with a given event.
func (irc *Connection) RunCallbacks(event *Event) {
	msg := event.Message()
	if event.Code == "PRIVMSG" && len(msg) > 0 && msg[0] == '\x01' {
		event.Code = "CTCP" //Unknown CTCP

		if i := strings.LastIndex(msg, "\x01"); i > -1 {
			msg = msg[1:i]
		}

		if msg == "VERSION" {
			event.Code = "CTCP_VERSION"

		} else if msg == "TIME" {
			event.Code = "CTCP_TIME"

		} else if msg[0:4] == "PING" {
			event.Code = "CTCP_PING"

		} else if msg == "USERINFO" {
			event.Code = "CTCP_USERINFO"

		} else if msg == "CLIENTINFO" {
			event.Code = "CTCP_CLIENTINFO"

		} else if msg[0:6] == "ACTION" {
			event.Code = "CTCP_ACTION"
			msg = msg[7:]
		}
		event.Arguments[len(event.Arguments)-1] = msg
	}

	if callbacks, ok := irc.events[event.Code]; ok {
		if irc.VerboseCallbackHandler {
			irc.Log.Printf("%v (%v) >> %#v\n", event.Code, len(callbacks), event)
		}

		for _, callback := range callbacks {
			go callback(event)
		}
	} else if irc.VerboseCallbackHandler {
		irc.Log.Printf("%v (0) >> %#v\n", event.Code, event)
	}

	if callbacks, ok := irc.events["*"]; ok {
		if irc.VerboseCallbackHandler {
			irc.Log.Printf("Wildcard %v (%v) >> %#v\n", event.Code, len(callbacks), event)
		}

		for _, callback := range callbacks {
			go callback(event)
		}
	}
}

func (irc *Connection) setupCallbacks() {
	irc.events = make(map[string]map[string]func(*Event))

	//Handle ping events
	irc.AddCallback("PING", func(e *Event) { irc.SendRaw("PONG :" + e.Message()) })

	//Version handler
	irc.AddCallback("CTCP_VERSION", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01VERSION %s\x01", e.Nick, irc.Version)
	})

	irc.AddCallback("CTCP_USERINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01USERINFO %s\x01", e.Nick, irc.user)
	})

	irc.AddCallback("CTCP_CLIENTINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01CLIENTINFO PING VERSION TIME USERINFO CLIENTINFO\x01", e.Nick)
	})

	irc.AddCallback("CTCP_TIME", func(e *Event) {
		ltime := time.Now()
		irc.SendRawf("NOTICE %s :\x01TIME %s\x01", e.Nick, ltime.String())
	})

	irc.AddCallback("CTCP_PING", func(e *Event) { irc.SendRawf("NOTICE %s :\x01%s\x01", e.Nick, e.Message) })

	irc.AddCallback("437", func(e *Event) {
		irc.nickcurrent = irc.nickcurrent + "_"
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	irc.AddCallback("433", func(e *Event) {
		if len(irc.nickcurrent) > 8 {
			irc.nickcurrent = "_" + irc.nickcurrent

		} else {
			irc.nickcurrent = irc.nickcurrent + "_"
		}
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	irc.AddCallback("PONG", func(e *Event) {
		ns, _ := strconv.ParseInt(e.Message(), 10, 64)
		delta := time.Duration(time.Now().UnixNano() - ns)
		if irc.Debug {
			irc.Log.Printf("Lag: %vs\n", delta)
		}
	})

	irc.AddCallback("NICK", func(e *Event) {
		if e.Nick == irc.nick {
			irc.nickcurrent = e.Message()
		}
	})

	irc.AddCallback("001", func(e *Event) {
		irc.nickcurrent = e.Arguments[0]
	})
}
