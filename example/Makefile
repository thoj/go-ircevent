all: $(GOARCH)

clean: clean_$(GOARCH)
	rm test

386:	
	8g test.go
	8l -o test test.8

amd64:
	6g test.go
	6l -o test test.6

arm:
	5g test.go
	5l -o test test.5

clean_amd64:
	rm *.6

clean_386:
	rm *.8

clean_arm:
	rm *.5

