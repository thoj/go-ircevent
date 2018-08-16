package irc

import (
	"context"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Register a callback to a connection and event code. A callback is a function
// which takes only an Event pointer as parameter. Valid event codes are all
// IRC/CTCP commands and error/response codes. To register a callback for all
// events pass "*" as the event code. This function returns the ID of the
// registered callback for later management.
func (irc *Connection) AddCallback(eventcode string, callback func(*Event)) int {
	eventcode = strings.ToUpper(eventcode)
	id := 0

	irc.eventsMutex.Lock()
	_, ok := irc.events[eventcode]
	if !ok {
		irc.events[eventcode] = make(map[int]func(*Event))
		id = 0
	} else {
		id = len(irc.events[eventcode])
	}
	irc.events[eventcode][id] = callback
	irc.eventsMutex.Unlock()
	return id
}

// Remove callback i (ID) from the given event code. This functions returns
// true upon success, false if any error occurs.
func (irc *Connection) RemoveCallback(eventcode string, i int) bool {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	event, ok := irc.events[eventcode]
	if ok {
		if _, ok := event[i]; ok {
			delete(irc.events[eventcode], i)
			irc.eventsMutex.Unlock()
			return true
		}
		irc.Log.Printf("Event found, but no callback found at id %d\n", i)
		irc.eventsMutex.Unlock()
		return false
	}

	irc.eventsMutex.Unlock()
	irc.Log.Println("Event not found")
	return false
}

// Remove all callbacks from a given event code. It returns true
// if given event code is found and cleared.
func (irc *Connection) ClearCallback(eventcode string) bool {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	_, ok := irc.events[eventcode]
	if ok {
		irc.events[eventcode] = make(map[int]func(*Event))
		irc.eventsMutex.Unlock()
		return true
	}
	irc.eventsMutex.Unlock()

	irc.Log.Println("Event not found")
	return false
}

// Replace callback i (ID) associated with a given event code with a new callback function.
func (irc *Connection) ReplaceCallback(eventcode string, i int, callback func(*Event)) {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	event, ok := irc.events[eventcode]
	irc.eventsMutex.Unlock()
	if ok {
		if _, ok := event[i]; ok {
			event[i] = callback
			return
		}
		irc.Log.Printf("Event found, but no callback found at id %d\n", i)
	}
	irc.Log.Printf("Event not found. Use AddCallBack\n")
}

// Execute all callbacks associated with a given event.
func (irc *Connection) RunCallbacks(event *Event) {
	msg := event.Message()
	if event.Code == "PRIVMSG" && len(msg) > 2 && msg[0] == '\x01' {
		event.Code = "CTCP" //Unknown CTCP

		if i := strings.LastIndex(msg, "\x01"); i > 0 {
			msg = msg[1:i]
		} else {
			irc.Log.Printf("Invalid CTCP Message: %s\n", strconv.Quote(msg))
			return
		}

		if msg == "VERSION" {
			event.Code = "CTCP_VERSION"

		} else if msg == "TIME" {
			event.Code = "CTCP_TIME"

		} else if strings.HasPrefix(msg, "PING") {
			event.Code = "CTCP_PING"

		} else if msg == "USERINFO" {
			event.Code = "CTCP_USERINFO"

		} else if msg == "CLIENTINFO" {
			event.Code = "CTCP_CLIENTINFO"

		} else if strings.HasPrefix(msg, "ACTION") {
			event.Code = "CTCP_ACTION"
			if len(msg) > 6 {
				msg = msg[7:]
			} else {
				msg = ""
			}
		}

		event.Arguments[len(event.Arguments)-1] = msg
	}

	irc.eventsMutex.Lock()
	callbacks := make(map[int]func(*Event))
	eventCallbacks, ok := irc.events[event.Code]
	id := 0
	if ok {
		for _, callback := range eventCallbacks {
			callbacks[id] = callback
			id++
		}
	}
	allCallbacks, ok := irc.events["*"]
	if ok {
		for _, callback := range allCallbacks {
			callbacks[id] = callback
			id++
		}
	}
	irc.eventsMutex.Unlock()

	if irc.VerboseCallbackHandler {
		irc.Log.Printf("%v (%v) >> %#v\n", event.Code, len(callbacks), event)
	}

	event.Ctx = context.Background()
	if irc.CallbackTimeout != 0 {
		event.Ctx, _ = context.WithTimeout(event.Ctx, irc.CallbackTimeout)
	}

	done := make(chan int)
	for id, callback := range callbacks {
		go func(id int, done chan<- int, cb func(*Event), event *Event) {
			start := time.Now()
			cb(event)
			select {
			case done <- id:
			case <-event.Ctx.Done(): // If we timed out, report how long until we eventually finished
				irc.Log.Printf("Canceled callback %s finished in %s >> %#v\n",
					getFunctionName(cb),
					time.Since(start),
					event,
				)
			}
		}(id, done, callback, event)
	}

	for len(callbacks) > 0 {
		select {
		case jobID := <-done:
			delete(callbacks, jobID)
		case <-event.Ctx.Done(): // context timed out!
			timedOutCallbacks := []string{}
			for _, cb := range callbacks { // Everything left here did not finish
				timedOutCallbacks = append(timedOutCallbacks, getFunctionName(cb))
			}
			irc.Log.Printf("Timeout while waiting for %d callback(s) to finish (%s)\n",
				len(callbacks),
				strings.Join(timedOutCallbacks, ", "),
			)
			return
		}
	}
}

func getFunctionName(f func(*Event)) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// Set up some initial callbacks to handle the IRC/CTCP protocol.
func (irc *Connection) setupCallbacks() {
	irc.events = make(map[string]map[int]func(*Event))

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

	irc.AddCallback("CTCP_PING", func(e *Event) { irc.SendRawf("NOTICE %s :\x01%s\x01", e.Nick, e.Message()) })

	// 437: ERR_UNAVAILRESOURCE "<nick/channel> :Nick/channel is temporarily unavailable"
	// Add a _ to current nick. If irc.nickcurrent is empty this cannot
	// work. It has to be set somewhere first in case the nick is already
	// taken or unavailable from the beginning.
	irc.AddCallback("437", func(e *Event) {
		// If irc.nickcurrent hasn't been set yet, set to irc.nick
		if irc.nickcurrent == "" {
			irc.nickcurrent = irc.nick
		}

		if len(irc.nickcurrent) > 8 {
			irc.nickcurrent = "_" + irc.nickcurrent
		} else {
			irc.nickcurrent = irc.nickcurrent + "_"
		}
		irc.SendRawf("NICK %s", irc.nickcurrent)
	})

	// 433: ERR_NICKNAMEINUSE "<nick> :Nickname is already in use"
	// Add a _ to current nick.
	irc.AddCallback("433", func(e *Event) {
		// If irc.nickcurrent hasn't been set yet, set to irc.nick
		if irc.nickcurrent == "" {
			irc.nickcurrent = irc.nick
		}

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
			irc.Log.Printf("Lag: %.3f s\n", delta.Seconds())
		}
	})

	// NICK Define a nickname.
	// Set irc.nickcurrent to the new nick actually used in this connection.
	irc.AddCallback("NICK", func(e *Event) {
		if e.Nick == irc.nick {
			irc.nickcurrent = e.Message()
		}
	})

	// 1: RPL_WELCOME "Welcome to the Internet Relay Network <nick>!<user>@<host>"
	// Set irc.nickcurrent to the actually used nick in this connection.
	irc.AddCallback("001", func(e *Event) {
		irc.Lock()
		irc.nickcurrent = e.Arguments[0]
		irc.Unlock()
	})
}
