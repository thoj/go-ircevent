// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	VERSION = "go-ircevent v2.0"
)

func (irc *Connection) readLoop() {
	br := bufio.NewReader(irc.socket)

	for {
		msg, err := br.ReadString('\n')
		if err != nil {
			irc.Error <- err
			break
		}

		irc.lastMessage = time.Now()
		msg = msg[0 : len(msg)-2] //Remove \r\n
		event := &Event{Raw: msg}
		if msg[0] == ':' {
			if i := strings.Index(msg, " "); i > -1 {
				event.Source = msg[1:i]
				msg = msg[i+1 : len(msg)]

			} else {
				irc.log.Printf("Misformed msg from server: %#s\n", msg)
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

	irc.syncreader <- true
}

func (irc *Connection) writeLoop() {
	b, ok := <-irc.pwrite
	for ok {
		if b == "" || irc.socket == nil {
			break
		}
		_, err := irc.socket.Write([]byte(b))
		if err != nil {
			irc.log.Printf("%s\n", err)
			irc.Error <- err
			break
		}

		b, ok = <-irc.pwrite
	}
	irc.syncwriter <- true
}

//Pings the server if we have not recived any messages for 5 minutes
func (irc *Connection) pingLoop() {
	irc.ticker = time.NewTicker(1 * time.Minute)   //Tick every minute.
	irc.ticker2 = time.NewTicker(15 * time.Minute) //Tick every 15 minutes.
	for {
		select {
		case <-irc.ticker.C:
			//Ping if we haven't recived anything from the server within 4 minutes
			if time.Since(irc.lastMessage) >= (4 * time.Minute) {
				irc.SendRawf("PING %d", time.Now().UnixNano())
			}
		case <-irc.ticker2.C:
			//Ping every 15 minutes.
			irc.SendRawf("PING %d", time.Now().UnixNano())
			//Try to recapture nickname if it's not as configured.
			if irc.nick != irc.nickcurrent {
				irc.nickcurrent = irc.nick
				irc.SendRawf("NICK %s", irc.nick)
			}
		case <-irc.endping:
			irc.ticker.Stop()
			irc.ticker2.Stop()
			break
		}
	}
	irc.syncpinger <- true
}

func (irc *Connection) Cycle() {
	irc.SendRaw("QUIT")
	irc.Reconnect()
}

func (irc *Connection) Quit() {
	irc.quitting = true
	irc.SendRaw("QUIT")
}

func (irc *Connection) Join(channel string) {
	irc.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}

func (irc *Connection) Part(channel string) {
	irc.pwrite <- fmt.Sprintf("PART %s\r\n", channel)
}

func (irc *Connection) Notice(target, message string) {
	irc.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (irc *Connection) Privmsg(target, message string) {
	irc.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}

func (irc *Connection) SendRaw(message string) {
	irc.log.Printf("--> %s\n", message)
	irc.pwrite <- fmt.Sprintf("%s\r\n", message)
}

func (irc *Connection) SendRawf(format string, a ...interface{}) {
	irc.SendRaw(fmt.Sprintf(format, a...))
}

func (irc *Connection) GetNick() string {
	return irc.nickcurrent
}

func (irc *Connection) Reconnect() error {
	close(irc.pwrite)
	close(irc.pread)
	irc.endping <- true
	irc.log.Printf("Syncing Threads\n")
	irc.log.Printf("Syncing Reader\n")
	<-irc.syncreader
	irc.log.Printf("Syncing Writer\n")
	<-irc.syncwriter
	irc.log.Printf("Syncing Pinger\n")
	<-irc.syncpinger
	irc.log.Printf("Syncing Threads Done\n")
	for {
		irc.log.Printf("Reconnecting to %s\n", irc.server)
		var err error
		irc.Connect(irc.server)
		if err == nil {
			break
		}
		irc.log.Printf("Error: %s\n", err)
	}
	return nil
}

func (irc *Connection) Loop() {
	for !irc.quitting {
		e := <-irc.Error
		if irc.quitting {
			break
		}
		irc.log.Printf("Error: %s\n", e)
		irc.Reconnect()
	}

	close(irc.pwrite)
	close(irc.pread)
	irc.endping <- true
	<-irc.syncreader
	<-irc.syncwriter
	<-irc.syncpinger
}

func (irc *Connection) Connect(server string) error {
	irc.server = server
	var err error
	irc.log.Printf("Connecting to %s\n", irc.server)
	if irc.UseSSL {
		irc.socket, err = tls.Dial("tcp", irc.server, irc.SSLConfig)
	} else {
		irc.socket, err = net.Dial("tcp", irc.server)
	}
	if err != nil {
		return err
	}
	irc.log.Printf("Connected to %s (%s)\n", irc.server, irc.socket.RemoteAddr())
	return irc.postConnect()
}

func (irc *Connection) postConnect() error {
	irc.pread = make(chan string, 100)
	irc.pwrite = make(chan string, 100)
	irc.Error = make(chan error, 10)
	irc.syncreader = make(chan bool)
	irc.syncwriter = make(chan bool)
	irc.syncpinger = make(chan bool)
	irc.endping = make(chan bool)
	go irc.readLoop()
	go irc.writeLoop()
	go irc.pingLoop()

	if len(irc.Password) > 0 {
		irc.pwrite <- fmt.Sprintf("PASS %s\r\n", irc.Password)
	}
	irc.pwrite <- fmt.Sprintf("NICK %s\r\n", irc.nick)
	irc.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", irc.user, irc.user)
	return nil
}

func IRC(nick, user string) *Connection {
	irc := new(Connection)
	irc.registered = false
	irc.pread = make(chan string, 100)
	irc.pwrite = make(chan string, 100)
	irc.Error = make(chan error)
	irc.nick = nick
	irc.user = user
	irc.VerboseCallbackHandler = false
	irc.log = log.New(os.Stdout, "", log.LstdFlags)
	irc.setupCallbacks()
	return irc
}
