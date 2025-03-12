init:
	go mod init main
	go get cloud.google.com/go
	go get cloud.google.com/go/pubsub
	go mod tidy

build: main.go
	go build -o main main.go

run: build 
	./main

clean: 
	@-rm -rf main 

.PHONY: init run clean
