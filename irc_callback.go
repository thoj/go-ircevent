// irc_callback.go - corrected version
// Copyright 2009 Thomas Jager <mail@jager.no>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"context"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CallbackID is a tuple type for uniquely identifying callbacks.
type CallbackID struct {
	EventCode string
	ID        int
}

// AddCallback registers a callback to a connection and event code.
// A callback is a function which takes only an Event pointer as a parameter.
// Valid event codes are all IRC/CTCP commands and error/response codes.
// To register a callback for all events, pass "*" as the event code.
// This function returns the ID of the registered callback for later management.
func (irc *Connection) AddCallback(eventcode string, callback func(*Event)) int {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	defer irc.eventsMutex.Unlock()

	if irc.events == nil {
		irc.events = make(map[string]map[int]func(*Event))
	}

	if _, ok := irc.events[eventcode]; !ok {
		irc.events[eventcode] = make(map[int]func(*Event))
	}
	id := irc.idCounter
	irc.idCounter++
	irc.events[eventcode][id] = callback
	return id
}

// RemoveCallback removes callback i (ID) from the given event code.
// This function returns true upon success, false if any error occurs.
func (irc *Connection) RemoveCallback(eventcode string, i int) bool {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	defer irc.eventsMutex.Unlock()

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			delete(event, i)
			return true
		}
		irc.Log.Printf("Event found, but no callback found at id %d\n", i)
		return false
	}

	irc.Log.Println("Event not found")
	return false
}

// ClearCallback removes all callbacks from a given event code.
// It returns true if the given event code is found and cleared.
func (irc *Connection) ClearCallback(eventcode string) bool {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	defer irc.eventsMutex.Unlock()

	if _, ok := irc.events[eventcode]; ok {
		irc.events[eventcode] = make(map[int]func(*Event))
		return true
	}

	irc.Log.Println("Event not found")
	return false
}

// ReplaceCallback replaces callback i (ID) associated with a given event code with a new callback function.
func (irc *Connection) ReplaceCallback(eventcode string, i int, callback func(*Event)) {
	eventcode = strings.ToUpper(eventcode)

	irc.eventsMutex.Lock()
	defer irc.eventsMutex.Unlock()

	if event, ok := irc.events[eventcode]; ok {
		if _, ok := event[i]; ok {
			event[i] = callback
			return
		}
		irc.Log.Printf("Event found, but no callback found at id %d\n", i)
		return
	}
	irc.Log.Printf("Event not found. Use AddCallback\n")
}

// RunCallbacks executes all callbacks associated with a given event.
func (irc *Connection) RunCallbacks(event *Event) {
	msg := event.Message()
	if event.Code == "PRIVMSG" && len(msg) > 2 && msg[0] == '\x01' {
		event.Code = "CTCP" // Unknown CTCP

		if i := strings.LastIndex(msg, "\x01"); i > 0 {
			msg = msg[1:i]
		} else {
			irc.Log.Printf("Invalid CTCP Message: %s\n", strconv.Quote(msg))
			return
		}

		switch {
		case msg == "VERSION":
			event.Code = "CTCP_VERSION"
		case msg == "TIME":
			event.Code = "CTCP_TIME"
		case strings.HasPrefix(msg, "PING"):
			event.Code = "CTCP_PING"
		case msg == "USERINFO":
			event.Code = "CTCP_USERINFO"
		case msg == "CLIENTINFO":
			event.Code = "CTCP_CLIENTINFO"
		case strings.HasPrefix(msg, "ACTION"):
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
	if eventCallbacks, ok := irc.events[event.Code]; ok {
		for id, callback := range eventCallbacks {
			callbacks[id] = callback
		}
	}
	if allCallbacks, ok := irc.events["*"]; ok {
		for id, callback := range allCallbacks {
			callbacks[id] = callback
		}
	}
	irc.eventsMutex.Unlock()

	if irc.VerboseCallbackHandler {
		irc.Log.Printf("%v (%v) >> %#v\n", event.Code, len(callbacks), event)
	}

	event.Ctx = context.Background()
	if irc.CallbackTimeout != 0 {
		var cancel context.CancelFunc
		event.Ctx, cancel = context.WithTimeout(event.Ctx, irc.CallbackTimeout)
		defer cancel()
	}

	done := make(chan int)
	for id, callback := range callbacks {
		go func(id int, done chan<- int, cb func(*Event), event *Event) {
			start := time.Now()
			cb(event)
			select {
			case done <- id:
			case <-event.Ctx.Done():
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
		case <-event.Ctx.Done():
			timedOutCallbacks := []string{}
			for _, cb := range callbacks {
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

// setupCallbacks sets up some initial callbacks to handle the IRC/CTCP protocol.
func (irc *Connection) setupCallbacks() {
	irc.events = make(map[string]map[int]func(*Event))

	// Handle PING events
	irc.AddCallback("PING", func(e *Event) {
		irc.SendRaw("PONG :" + e.Message())
	})

	// Version handler
	irc.AddCallback("CTCP_VERSION", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01VERSION %s\x01", e.Nick, irc.Version)
	})

	// Userinfo handler
	irc.AddCallback("CTCP_USERINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01USERINFO %s\x01", e.Nick, irc.user)
	})

	// Clientinfo handler
	irc.AddCallback("CTCP_CLIENTINFO", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01CLIENTINFO PING VERSION TIME USERINFO CLIENTINFO\x01", e.Nick)
	})

	// Time handler
	irc.AddCallback("CTCP_TIME", func(e *Event) {
		ltime := time.Now()
		irc.SendRawf("NOTICE %s :\x01TIME %s\x01", e.Nick, ltime.String())
	})

	// Ping handler
	irc.AddCallback("CTCP_PING", func(e *Event) {
		irc.SendRawf("NOTICE %s :\x01%s\x01", e.Nick, e.Message())
	})

	// Handle nickname in use (433)
	irc.AddCallback("433", func(e *Event) {
		irc.Lock()
		defer irc.Unlock()
		if !irc.fullyConnected {
			if irc.nickcurrent == "" {
				irc.nickcurrent = irc.nick
			}
			irc.modifyNick()
			irc.SendRawf("NICK %s", irc.nickcurrent)
		}
	})

	// Handle unavailable resource (437)
	irc.AddCallback("437", func(e *Event) {
		irc.Lock()
		defer irc.Unlock()
		if !irc.fullyConnected {
			if irc.nickcurrent == "" {
				irc.nickcurrent = irc.nick
			}
			irc.modifyNick()
			irc.SendRawf("NICK %s", irc.nickcurrent)
		}
	})

	// Handle no nickname given (431)
	irc.AddCallback("431", func(e *Event) {
		irc.Lock()
		defer irc.Unlock()
		if !irc.fullyConnected {
			if irc.nickcurrent == "" {
				irc.nickcurrent = irc.nick
			}
			irc.modifyNick()
			irc.SendRawf("NICK %s", irc.nickcurrent)
		}
	})

	// Handle erroneous nickname (432)
	irc.AddCallback("432", func(e *Event) {
		irc.Lock()
		defer irc.Unlock()
		if !irc.fullyConnected {
			if irc.nickcurrent == "" {
				irc.nickcurrent = irc.nick
			}
			// Add prefix 'Err' to try a different nickname
			irc.nickcurrent = "Err" + irc.nickcurrent
			irc.SendRawf("NICK %s", irc.nickcurrent)
		}
	})

	// Handle nickname collision (436)
	irc.AddCallback("436", func(e *Event) {
		irc.Lock()
		defer irc.Unlock()
		if !irc.fullyConnected {
			if irc.nickcurrent == "" {
				irc.nickcurrent = irc.nick
			}
			irc.modifyNick()
			irc.SendRawf("NICK %s", irc.nickcurrent)
		}
	})

	// Handle restricted nickname (484)
	irc.AddCallback("484", func(e *Event) {
		irc.Lock()
		defer irc.Unlock()
		if !irc.fullyConnected {
			// Keep the current nickname and do not attempt to change it further
		}
	})

	// Handle PONG responses
	irc.AddCallback("PONG", func(e *Event) {
		ns, _ := strconv.ParseInt(e.Message(), 10, 64)
		delta := time.Duration(time.Now().UnixNano() - ns)
		if irc.Debug {
			irc.Log.Printf("Lag: %.3f s\n", delta.Seconds())
		}
	})

	// Handle NICK changes
	irc.AddCallback("NICK", func(e *Event) {
		if e.Nick == irc.nick {
			irc.nickcurrent = e.Message()
		}
	})

	// Set fullyConnected to true on successful connection (001)
	irc.AddCallback("001", func(e *Event) {
		irc.Lock()
		irc.nickcurrent = e.Arguments[0]
		irc.fullyConnected = true
		irc.Unlock()
	})
}

// modifyNick modifies the current nickname to try a different one.
func (irc *Connection) modifyNick() {
	if len(irc.nickcurrent) > 8 {
		irc.nickcurrent = "_" + irc.nickcurrent
	} else {
		irc.nickcurrent = irc.nickcurrent + "_"
	}
}
