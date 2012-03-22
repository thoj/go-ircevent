include $(GOROOT)/src/Make.inc

TARG=github.com/lye/cleanirc
GOFILES=\
	src/irc.go \
	src/irc_struct.go \
	src/irc_callback.go

include $(GOROOT)/src/Make.pkg 
