package main

import (
	"fmt"
	"github.com/thoja/go-ircevent"
	"sort"
	"time"
)

const channel = "#ggnet"
const serverssl = "irc.homelien.no:6667"

func main() {
	ircnick1 := "blatibalt1"
	irccon := irc.IRC(ircnick1, "blatiblat")
	irccon.VerboseCallbackHandler = true
	irccon.Debug = true
	irccon.AddCallback("001", func(e *irc.Event) { irccon.Join(channel) })
	irccon.AddCallback("366", func(e *irc.Event) {})
	irccon.SetupNickTrack()
	err := irccon.Connect(serverssl)
	if err != nil {
		fmt.Printf("Err %s", err)
		return
	}
	go func() {
		t := time.NewTicker(30 * time.Second)
		for {
			<-t.C
			var keys []string
			if _, ok := irccon.Channels[channel]; ok == true {
				for k, _ := range irccon.Channels[channel].Users {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				fmt.Printf("%d: ", len(keys))
				for _, k := range keys {
					fmt.Printf("(%s)%s ", irccon.Channels[channel].Users[k].Mode, k)
				}
				fmt.Printf("\n")
			}
		}
	}()
	irccon.Loop()
}
