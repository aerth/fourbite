bin: cmd/*/*.go
	mkdir -p bin
	cd bin && CGO_ENABLED=0 go build -ldflags '-w -s' -o . ../cmd/...
run: bin
	bin/4server
clean:
	${RM} -r bin
