.PHONY: build-and-use
build-and-use:
	@mkdir -p out
	go build -o out/dbtool ./cmd/dbtool/main.go
	cp -f out/dbtool ~/.bin/dbtool
	chmod +x ~/.bin/dbtool
