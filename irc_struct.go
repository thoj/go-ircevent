// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"os"
	"net"
)

type IRCConnection struct {
	socket        net.Conn
	pread, pwrite chan string
	Error         chan os.Error
	nick          string
	user          string
	registered    bool
	server        string

	events map[string][]func(*IRCEvent)
	
	lastMessage int64;
	ticker <-chan int64;
	ticker2 <-chan int64;
}

type IRCEvent struct {
	Code    string
	Message string
	Raw     string
	Nick    string //<nick>
	Host    string //<nick>!<usr>@<host>
	Source  string //<host>
	User    string //<usr>

	Arguments []string
}
