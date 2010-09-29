include $(GOROOT)/src/Make.${GOARCH}

TARG=irc
GOFILES=irc.go irc_struct.go irc_callback.go

include $(GOROOT)/src/Make.pkg 
