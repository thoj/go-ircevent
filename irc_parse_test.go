package irc

import (
	"testing"
)

func checkResult(t *testing.T, event *Event) {
	if event.Nick != "nick" {
		t.Fatal("Parse failed: nick")
	}
	if event.User != "~user" {
		t.Fatal("Parse failed: user")
	}
	if event.Code != "PRIVMSG" {
		t.Fatal("Parse failed: code")
	}
	if event.Arguments[0] != "#channel" {
		t.Fatal("Parse failed: channel")
	}
	if event.Arguments[1] != "message text" {
		t.Fatal("Parse failed: message")
	}
}

func TestParse(t *testing.T) {
	event, err := parseToEvent(":nick!~user@host PRIVMSG #channel :message text")
	if err != nil {
		t.Fatal("Parse PRIVMSG failed")
	}
	checkResult(t, event)
}

func TestParseTags(t *testing.T) {
	event, err := parseToEvent("@tag;+tag2=raw+:=,escaped\\:\\s\\\\ :nick!~user@host PRIVMSG #channel :message text")
	if err != nil {
		t.Fatal("Parse PRIVMSG with tags failed")
	}
	checkResult(t, event)
	t.Logf("%s", event.Tags)
	if _, ok := event.Tags["tag"]; !ok {
		t.Fatal("Parsing value-less tag failed")
	}
	if event.Tags["+tag2"] != "raw+:=,escaped; \\" {
		t.Fatal("Parsing tag failed")
	}
}
