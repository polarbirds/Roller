TOKEN := $(shell cat token.txt)

build:
	go build cmd/main/main.go

windows:
	GOOS=windows GOARCH=amd64 go build cmd/main/main.go

linux:
	GOOS=linux GOARCH=amd64 go build cmd/main/main.go

run:
	./main -t ${TOKEN}

run-windows:
	main.exe -t ${TOKEN}

