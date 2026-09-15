VERSION ?= 1.0.0
ARCH   ?= amd64

.PHONY: build deb clean

build:
	CGO_ENABLED=0 go build -o asterisk-pn .

deb: build
	./scripts/build-deb.sh $(VERSION) $(ARCH)

clean:
	rm -f asterisk-pn *.deb
