// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"crypto/tls"
	"log"
	"net"
	"time"
)

type Connection struct {
	Debug     bool
	Error     chan error
	Password  string
	UseTLS    bool
	TLSConfig *tls.Config
	Version   string
	Timeout   time.Duration
	PingFreq  time.Duration
	KeepAlive time.Duration

	socket                             net.Conn
	netsock                            net.Conn
	pread, pwrite                      chan string
	readerExit, writerExit, pingerExit chan bool
	endping, endread, endwrite         chan bool

	nick        string //The nickname we want.
	nickcurrent string //The nickname we currently have.
	user        string
	registered  bool
	server      string
	events      map[string][]func(*Event)

	lastMessage time.Time

	VerboseCallbackHandler bool
	Log                    *log.Logger

	stopped bool
}

type Event struct {
	Code      string
	Raw       string
	Nick      string //<nick>
	Host      string //<nick>!<usr>@<host>
	Source    string //<host>
	User      string //<usr>
	Arguments []string
}

// Convenience func to get the last arg, now that the Message field is gone
func (e *Event) Message() string {
	if len(e.Arguments) == 0 {
		return ""
	}
	return e.Arguments[len(e.Arguments)-1]
}
