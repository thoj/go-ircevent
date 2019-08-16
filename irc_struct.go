// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"regexp"
	"sync"
	"time"

	"golang.org/x/text/encoding"
)

type Connection struct {
	sync.Mutex
	sync.WaitGroup
	Debug            bool
	Error            chan error
	WebIRC           string
	Password         string
	UseTLS           bool
	UseSASL          bool
	RequestCaps      []string
	AcknowledgedCaps []string
	SASLLogin        string
	SASLPassword     string
	SASLMech         string
	TLSConfig        *tls.Config
	Version          string
	Timeout          time.Duration
	CallbackTimeout  time.Duration
	PingFreq         time.Duration
	KeepAlive        time.Duration
	Server           string
	Encoding         encoding.Encoding

	RealName string // The real name we want to display.
	// If zero-value defaults to the user.

	socket net.Conn
	pwrite chan string
	end    chan struct{}

	nick        string //The nickname we want.
	nickcurrent string //The nickname we currently have.
	user        string
	registered  bool
	events      map[string]map[int]func(*Event)
	eventsMutex sync.Mutex

	QuitMessage      string
	lastMessage      time.Time
	lastMessageMutex sync.Mutex

	VerboseCallbackHandler bool
	Log                    *log.Logger

	stopped bool
	quit    bool

	Channels map[string]Channel
}

// A struct to represent an event.
type Event struct {
	Code       string
	Raw        string
	Nick       string //<nick>
	Host       string //<nick>!<usr>@<host>
	Source     string //<host>
	User       string //<usr>
	Arguments  []string
	Tags       map[string]string
	Connection *Connection
	Ctx        context.Context
}

// Retrieve the last message from Event arguments.
// This function leaves the arguments untouched and
// returns an empty string if there are none.
func (e *Event) Message() string {
	if len(e.Arguments) == 0 {
		return ""
	}
	return e.Arguments[len(e.Arguments)-1]
}

// https://stackoverflow.com/a/10567935/6754440
// Regex of IRC formatting.
var ircFormat = regexp.MustCompile(`[\x02\x1F\x0F\x16\x1D\x1E]|\x03(\d\d?(,\d\d?)?)?`)

// Retrieve the last message from Event arguments, but without IRC formatting (color.
// This function leaves the arguments untouched and
// returns an empty string if there are none.
func (e *Event) MessageWithoutFormat() string {
	if len(e.Arguments) == 0 {
		return ""
	}
	return ircFormat.ReplaceAllString(e.Arguments[len(e.Arguments)-1], "")
}
