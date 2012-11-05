// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
	"crypto/tls"
)

const (
	VERSION = "cleanirc v1.0"
)

var error_ bool

func readLoop(irc *Connection) {
	br := bufio.NewReader(irc.socket)

	for !irc.reconnecting {
		msg, err := br.ReadString('\n')
		if err != nil {
			irc.Error <-err
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

	irc.syncreader <-true
}

func writeLoop(irc *Connection) {
	b, ok := <-irc.pwrite
	for !irc.reconnecting && ok {
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
	irc.syncwriter <-true
}

//Pings the server if we have not recived any messages for 5 minutes
func pingLoop(i *Connection) {
	i.ticker = time.Tick(1 * time.Minute)   //Tick every minute.
	i.ticker2 = time.Tick(15 * time.Minute) //Tick every 15 minutes.
	for {
		select {
		case <-i.ticker:
			//Ping if we haven't recived anything from the server within 4 minutes
			if time.Since(i.lastMessage) >= (4 * time.Minute) {
				i.SendRawf("PING %d", time.Now().UnixNano())
			}
		case <-irc.ticker2:
			//Ping every 15 minutes.
			i.SendRawf("PING %d", time.Now().UnixNano())
			//Try to recapture nickname if it's not as configured.
			if i.nick != i.nickcurrent {
				i.nickcurrent = i.nick
				i.SendRawf("NICK %s", i.nick)
			}
		}
	}
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

func (i *Connection) Reconnect() error {
	close(i.pwrite)
	close(i.pread)
	<-i.syncreader
	<-i.syncwriter
	for {
		i.log.Printf("Reconnecting to %s\n", i.server)
		var err error
		irc.Connect(irc.server)
		if err == nil {
			break
		}
		i.log.Printf("Error: %s\n", err)
	}
        error_ = false
	return nil
}

func (i *Connection) Loop() {
	for !i.quitting {
		e := <-i.Error
		if i.quitting {
			break
		}
		i.log.Printf("Error: %s\n", e)
		error_ = true
		i.Reconnect()
	}

	close(irc.pwrite)
	close(irc.pread)

	<-irc.syncreader
	<-irc.syncwriter
}


func (irc *Connection) Connect(server string) error {
	irc.server = server
	irc.log.Printf("Connecting to %s\n", i.server)
	var err error
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
