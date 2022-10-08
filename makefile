bin: cmd/*/*.go
	mkdir -p bin
	cd bin && go build -o . ../cmd/...
clean:
	${RM} -r bin
