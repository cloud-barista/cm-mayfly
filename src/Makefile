default:
	go build -o ../bin/mayfly
linux-arm:
	GOOS=linux GOARCH=arm go build -o ../bin/mayfly
#windows 64bint
win:
	GOOS=windows GOARCH=amd64 go build -o ../bin/mayfly.exe
#windows 32bit
win86:
	GOOS=windows GOARCH=386 go build -o ../bin/mayfly.exe
mac:
	GOOS=darwin GOARCH=amd64 go build -o ../bin/mayfly
mac-arm:
	GOOS=darwin GOARCH=arm64 go build -o ../bin/mayfly
clean:
	rm -v ../bin/mayfly ../bin/mayfly.exe