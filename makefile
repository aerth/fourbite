bin: cmd/*/*.go
	mkdir -p bin
	cd bin && CGO_ENABLED=0 go build -ldflags '-w -s' -o . ../cmd/...
run: bin
	bin/4server
runindex: bin 4bytes-master.zip
	bin/4index
getsigs: 4bytes-master.zip runindex
4bytes-master.zip:
	wget -c -O 4bytes-master.zip https://github.com/ethereum-lists/4bytes/archive/refs/heads/master.zip
clean:
	${RM} -r bin
