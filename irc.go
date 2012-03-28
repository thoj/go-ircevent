// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
	"crypto/tls"
)

const (
	VERSION = "cleanirc v1.0"
)

func (irc *IRCConnection) readLoop() {
	br := bufio.NewReader(irc.socket)

	for !irc.reconnecting {
		msg, err := br.ReadString('\n')
		if err != nil {
			irc.Error <-err
			break
		}

		irc.lastMessage = time.Now()
		msg = msg[0 : len(msg)-2] //Remove \r\n
		event := &IRCEvent{Raw: msg}

		if msg[0] == ':' {
			if i := strings.Index(msg, " "); i > -1 {
				event.Source = msg[1:i]
				msg = msg[i+1 : len(msg)]

			} else {
				irc.log(fmt.Sprintf("Misformed msg from server: %#s\n", msg))
			}

			if i, j := strings.Index(event.Source, "!"), strings.Index(event.Source, "@"); i > -1 && j > -1 {
				event.Nick = event.Source[0:i]
				event.User = event.Source[i+1 : j]
				event.Host = event.Source[j+1 : len(event.Source)]
			}
		}

		args := strings.SplitN(msg, " :", 2)
		if len(args) > 1 {
			event.Message = args[1]
		}

		args = strings.Split(args[0], " ")
		event.Code = strings.ToUpper(args[0])

		if len(args) > 1 {
			event.Arguments = args[1:len(args)]
		}
		/* XXX: len(args) == 0: args should be empty */

		irc.RunCallbacks(event)
	}

	irc.syncreader <-true
}

func (irc *IRCConnection) writeLoop() {
	b, ok := <-irc.pwrite

	for !irc.reconnecting && ok {
		if b == "" || irc.socket == nil {
			break
		}

		_, err := irc.socket.Write([]byte(b))
		if err != nil {
			irc.Error <-err
			break
		}

		b, ok = <-irc.pwrite
	}
	irc.syncwriter <-true
}

//Pings the server if we have not recived any messages for 5 minutes
func (irc *IRCConnection) pingLoop() {
	irc.ticker = time.Tick(1 * time.Minute)   //Tick every minute.
	irc.ticker2 = time.Tick(15 * time.Minute) //Tick every 15 minutes.

	for {
		select {
		case <-irc.ticker:
			//Ping if we haven't received anything from the server within 4 minutes
			if time.Since(irc.lastMessage) >= (4 * time.Minute) {
				irc.SendRaw(fmt.Sprintf("PING %d", time.Now().UnixNano()))
			}

		case <-irc.ticker2:
			//Ping every 15 minutes.
			irc.SendRaw(fmt.Sprintf("PING %d", time.Now().UnixNano()))

			//Try to recapture nickname if it's not as configured.
			if irc.nick != irc.nickcurrent {
				irc.nickcurrent = irc.nick
				irc.SendRaw(fmt.Sprintf("NICK %s", irc.nick))
			}
		}
	}
}

func (irc *IRCConnection) Cycle() {
	irc.SendRaw("QUIT")
	irc.Reconnect()
}

func (irc *IRCConnection) Quit() {
	irc.quitting = true
	irc.SendRaw("QUIT")
}

func (irc *IRCConnection) Join(channel string) {
	irc.pwrite <-fmt.Sprintf("JOIN %s\r\n", channel)
}

func (irc *IRCConnection) Part(channel string) {
	irc.pwrite <-fmt.Sprintf("PART %s\r\n", channel)
}

func (irc *IRCConnection) Notice(target, message string) {
	irc.pwrite <-fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (irc *IRCConnection) Privmsg(target, message string) {
	irc.pwrite <-fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}

func (irc *IRCConnection) SendRaw(message string) {
	irc.log(fmt.Sprintf("--> %s\n", message))
	irc.pwrite <-fmt.Sprintf("%s\r\n", message)
}

func (irc *IRCConnection) Reconnect() error {
	irc.reconnecting = true

	close(irc.pwrite)
	close(irc.pread)

	<-irc.syncreader
	<-irc.syncwriter

	for {
		irc.log(fmt.Sprintf("Reconnecting to %s\n", irc.server))

		var err error
		irc.socket, err = net.Dial("tcp", irc.server)
		if err == nil {
			break
		}

		irc.log(fmt.Sprintf("Error: %s\n", err))
	}

	irc.reconnecting = false

	irc.log(fmt.Sprintf("Connected to %s (%s)\n", irc.server, irc.socket.RemoteAddr()))

	go irc.readLoop()
	go irc.writeLoop()

	irc.pwrite <-fmt.Sprintf("NICK %s\r\n", irc.nick)
	irc.pwrite <-fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", irc.user, irc.user)

	return nil
}

func (irc *IRCConnection) Loop() {
	for !irc.quitting {
		e := <-irc.Error

		if irc.quitting {
			break
		}

		irc.log(fmt.Sprintf("Error: %s\n", e))
		irc.Reconnect()
	}

	close(irc.pwrite)
	close(irc.pread)

	<-irc.syncreader
	<-irc.syncwriter
}

func (irc *IRCConnection) postConnect() error {
	irc.pread = make(chan string, 100)
	irc.pwrite = make(chan string, 100)
	irc.Error = make(chan error, 10)
	irc.syncreader = make(chan bool)
	irc.syncwriter = make(chan bool)

	go irc.readLoop()
	go irc.writeLoop()
	go irc.pingLoop()

	if len(irc.Password) > 0 {
		irc.pwrite <-fmt.Sprintf("PASS %s\r\n", irc.Password)
	}

	irc.pwrite <-fmt.Sprintf("NICK %s\r\n", irc.nick)
	irc.pwrite <-fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", irc.user, irc.user)
	return nil
}

func (irc *IRCConnection) Connect(server string) error {
	irc.server = server
	irc.log(fmt.Sprintf("Connecting to %s\n", irc.server))

	var err error
	irc.socket, err = net.Dial("tcp", irc.server)
	if err != nil {
		return err
	}

	irc.log(fmt.Sprintf("Connected to %s (%s)\n", irc.server, irc.socket.RemoteAddr()))
	return irc.postConnect()
}

func (irc *IRCConnection) ConnectSSL(server string) error {
	irc.server = server
	irc.log(fmt.Sprintf("Connecting to %s over SSL\n", irc.server))

	var err error
	irc.socket, err = tls.Dial("tcp", irc.server, irc.SSLConfig)

	if err != nil {
		return err
	}

	irc.log(fmt.Sprintf("Connected to %s (%s) over SSL\n", irc.server, irc.socket.RemoteAddr()))

	return irc.postConnect()
}

func (irc *IRCConnection) log(msg string) {
	if irc.Log != nil {
		irc.Log <-msg
	}
}

/* XXX: Change ctor name */
func IRC(nick, user string) *IRCConnection {
	irc := new(IRCConnection)
	irc.registered = false
	irc.pread = make(chan string, 100)
	irc.pwrite = make(chan string, 100)
	irc.Error = make(chan error)
	irc.nick = nick
	irc.user = user
	irc.VerboseCallbackHandler = true
	irc.setupCallbacks()
	return irc
}
