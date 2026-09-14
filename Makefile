VERSION ?= 1.0.0
ARCH   ?= amd64

.PHONY: build deb clean

build:
	go build -o asterisk-push-notify .

deb: build
	./scripts/build-deb.sh $(VERSION) $(ARCH)

clean:
	rm -f asterisk-push-notify *.deb
