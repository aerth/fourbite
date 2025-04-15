bin: cmd/*/*.go
	mkdir -p bin
	cd bin && CGO_ENABLED=0 go build -ldflags '-w -s' -o . ../cmd/...
clean:
	${RM} -r bin
run: bin
	bin/4server
runindex: bin 4bytes-master.zip
	bin/4index
getsigs: 4bytes-master.zip runindex # download the big zip

/tmp/siglist.txt: 4bytes-master.zip # output list of hex sigs
	unzip -l 4bytes-master.zip  | egrep '4bytes-master/signatures/........' | awk '{print $$4}' | sed 's@4bytes-master/signatures/@@g' >> /tmp/siglist.txt
testsigserver: /tmp/siglist.txt  # test each sig is available
	(while read LINE; do curl --fail  --no-progress-meter http://localhost:8081/4byte/$$LINE >/dev/null; if [ $$? -ne 0 ]; then exit 1; fi; done </tmp/siglist.txt)

4bytes-master.zip:
	wget -c -O 4bytes-master.zip https://github.com/ethereum-lists/4bytes/archive/refs/heads/master.zip
