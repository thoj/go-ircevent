include $(GOROOT)/src/Make.inc

OBJS := $(patsubst %.go,%.$O,$(wildcard *.go))
OUT := $(patsubst %.$O,%,$(OBJS))

all: $(OBJS)

%.$O: %.go
	$(GC) $<
	$(LD) -o $(patsubst %.$O,%,$@) $@

clean:
	rm -f *.$O $(OBJS) $(OUT)
