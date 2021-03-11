access_token=
soure_hut_user=
BIN_NAME := gembro
BIN := build/${BIN_NAME}
DEPS := $(shell find . -iname '*.go') go.mod go.sum
DESTDIR := ${HOME}/.local/bin
CGO_ENABLED := 0
GOOS := 
GOARCH :=

${BIN}: ${DEPS}
	CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build -o ${BIN}

.PHONY: install	
install: ${BIN}
	install -m 755 -d ${DESTDIR}
	install -m 755 -Cv ${BIN} ${DESTDIR}

.PHONY: clean
clean:
	! test -e ${BIN} || rm ${BIN}
	! test -e build || rmdir build

docs/site.tar.gz: docs/*.gmi
	cd docs && tar -cvz *.gmi > site.tar.gz

.PHONY: upload-site
upload-site: docs/site.tar.gz
	cd docs && curl --oauth2-bearer "$(access_token)" \
		-Fcontent=@site.tar.gz \
		-Fprotocol=GEMINI \
		https://pages.sr.ht/publish/${soure_hut_user}.srht.site 
