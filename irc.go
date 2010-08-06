// Copyright 2009 Thomas Jager <mail@jager.no>  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irc

import (
	"fmt"
	"net"
	"os"
	"bufio"
	"strings"
	"time"
)

const (
	VERSION = "GolangBOT v1.0"
)


func reader(irc *IRCConnection) {
	br := bufio.NewReader(irc.socket)
	for {
		msg, err := br.ReadString('\n')
		if err != nil {
			irc.Error <- err
			return
		}
		irc.lastMessage = time.Seconds()
		msg = msg[0 : len(msg)-2] //Remove \r\n
		event := &IRCEvent{Raw: msg}
		if msg[0] == ':' {
			if i := strings.Index(msg, " "); i > -1 {
				event.Source = msg[1:i]
				msg = msg[i+1 : len(msg)]
			} else {
				fmt.Printf("Misformed msg from server: %#s\n", msg)
			}
			if i, j := strings.Index(event.Source, "!"), strings.Index(event.Source, "@"); i > -1 && j > -1 {
				event.Nick = event.Source[0:i]
				event.User = event.Source[i+1 : j]
				event.Host = event.Source[j+1 : len(event.Source)]
			}
		}
		args := strings.Split(msg, " :", 2)
		if len(args) > 1 {
			event.Message = args[1]
		}
		args = strings.Split(args[0], " ", -1)
		event.Code = strings.ToUpper(args[0])
		if len(args) > 1 {
			event.Arguments = args[1:len(args)]
		}
		irc.RunCallbacks(event)
	}
}

func writer(irc *IRCConnection) {
	for {
		b := []byte(<-irc.pwrite)
		if b == nil {
			return
		}
		_, err := irc.socket.Write(b)
		if err != nil {
			fmt.Printf("%s\n", err)
			irc.Error <- err
			return
		}
	}
}

//Pings the server if we have not recived any messages for 5 minutes
func pinger(i *IRCConnection) {
	i.ticker = time.Tick(1000 * 1000 * 1000 * 60 * 4)   //Every 4 minutes
	i.ticker2 = time.Tick(1000 * 1000 * 1000 * 60 * 15) //Every 15 minutes
	for {
		select {
		case <-i.ticker:
			if time.Seconds()-i.lastMessage > 60*4 {
				i.SendRaw(fmt.Sprintf("PING %d", time.Nanoseconds()))
			}
		case <-i.ticker2:
			i.SendRaw(fmt.Sprintf("PING %d", time.Nanoseconds()))
		}
	}
}

func (irc *IRCConnection) Join(channel string) {
	irc.pwrite <- fmt.Sprintf("JOIN %s\r\n", channel)
}

func (irc *IRCConnection) Notice(target, message string) {
	irc.pwrite <- fmt.Sprintf("NOTICE %s :%s\r\n", target, message)
}

func (irc *IRCConnection) Privmsg(target, message string) {
	irc.pwrite <- fmt.Sprintf("PRIVMSG %s :%s\r\n", target, message)
}

func (irc *IRCConnection) SendRaw(message string) {
	fmt.Printf("--> %s\n", message)
	irc.pwrite <- fmt.Sprintf("%s\r\n", message)
}

func (i *IRCConnection) Reconnect() os.Error {
	for {
		fmt.Printf("Reconnecting to %s\n", i.server)
		var err os.Error
		i.socket, err = net.Dial("tcp", "", i.server)
		if err == nil {
			break
		}
		fmt.Printf("Error: %s\n", err)
	}
	fmt.Printf("Connected to %s (%s)\n", i.server, i.socket.RemoteAddr())
	go reader(i)
	go writer(i)
	i.pwrite <- fmt.Sprintf("NICK %s\r\n", i.nick)
	i.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", i.user, i.user)
	return nil
}

func (i *IRCConnection) Loop() {
	for {
		<-i.Error
		i.Reconnect()
	}
}

func (i *IRCConnection) Connect(server string) os.Error {
	i.server = server
	fmt.Printf("Connecting to %s\n", i.server)
	var err os.Error
	i.socket, err = net.Dial("tcp", "", i.server)
	if err != nil {
		return err
	}
	fmt.Printf("Connected to %s (%s)\n", i.server, i.socket.RemoteAddr())
	i.pread = make(chan string, 100)
	i.pwrite = make(chan string, 100)
	i.Error = make(chan os.Error, 10)
	go reader(i)
	go writer(i)
	go pinger(i)
	i.pwrite <- fmt.Sprintf("NICK %s\r\n", i.nick)
	i.pwrite <- fmt.Sprintf("USER %s 0.0.0.0 0.0.0.0 :%s\r\n", i.user, i.user)
	return nil
}

func IRC(nick string, user string) *IRCConnection {
	irc := new(IRCConnection)
	irc.registered = false
	irc.pread = make(chan string, 100)
	irc.pwrite = make(chan string, 100)
	irc.Error = make(chan os.Error)
	irc.nick = nick
	irc.user = user
	irc.setupCallbacks()
	return irc
}
