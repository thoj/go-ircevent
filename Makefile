include $(GOROOT)/src/Make.inc

TARG=irc
GOFILES=irc.go irc_struct.go irc_callback.go

include $(GOROOT)/src/Make.pkg 
