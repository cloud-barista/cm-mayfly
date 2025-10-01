.PHONY: default linux-arm win win86 mac mac-arm clean for-docker

# Default build commands
default: for-docker
	go build -o ./mayfly

# Build commands for Linux ARM
linux-arm:
	GOOS=linux GOARCH=arm go build -o ./mayfly

# Build commands for Windows 64bit
win:
	GOOS=windows GOARCH=amd64 go build -o ./mayfly.exe

# Build commands for Windows 32bit
win86:
	GOOS=windows GOARCH=386 go build -o ./mayfly.exe

# Build commands for macOS 64bit
mac:
	GOOS=darwin GOARCH=amd64 go build -o ./mayfly

# Build commands for macOS ARM64
mac-arm:
	GOOS=darwin GOARCH=arm64 go build -o ./mayfly

# CGO_ENABLED=0 - for Alpine Linux (issue #19)
# Using mayfly instead of curl for health checks on docker containers
for-docker:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./conf/docker/tool/mayfly


#Deleting all build files
clean:
	rm -v ./mayfly ./mayfly.exe