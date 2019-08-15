PROJECT_NAME:= "log-aggregation"

.PHONY: install clean build

S3_BUCKET=test-bucket
STACK_NAME=log-stack

install:
	go install $(GOPATH)/...

clean: 
	rm -rf ./src/main

build:
	GOOS=linux GOARCH=amd64 go build -o build/sendlog sendlog/sendlog.go
	GOOS=linux GOARCH=amd64 go build -o build/senderrorlog senderrorlog.go
	GOOS=linux GOARCH=amd64 go build -o build/alert alert.go
	zip sendlog.zip build/sendlog
	zip senderrorlog.zip build/senderrorlog
	zip alert.zip build/alert

