GOCMD = go
GOBUILD = $(GOCMD) build
GOINSTALL = $(GOCMD) install
GOCLEAN = $(GOCMD) clean
GOGET = $(GOCMD) get
FLAG_MODULE = GO111MODULE=on
FLAGS_SHARED = $(FLAG_MODULE) CGO_ENABLED=0 GOARCH=amd64
FLAGS_LINUX = $(FLAGS_SHARED) GOOS=linux
FLAGS_MAC = $(FLAGS_SHARED) GOOS=darwin
FLAGS_WIN = $(FLAGS_SHARED) GOOS=windows

VERSION = $(shell git describe --tags)
VER = $(shell git describe --tags --abbrev=0)
DATE = $(shell date -u '+%Y-%m-%d_%H:%M:%S%Z')

all: install

test: deps install
	$(FLAG_MODULE) go test ./...

test-build: deps build

deps:
	$(FLAG_MODULE) $(GOGET) github.com/pointlander/peg@fa48cc2; \
	$(FLAG_MODULE) $(GOGET) github.com/shurcooL/vfsgen@6a9ea43; \
	$(FLAG_MODULE) $(GOGET) github.com/spf13/cobra/cobra@7547e83; \
	$(FLAG_MODULE) $(GOGET) github.com/onsi/ginkgo/ginkgo@505cc35; \
	$(FLAG_MODULE) $(GOGET) github.com/onsi/gomega@ce690c5; \
	$(FLAG_MODULE) $(GOGET) github.com/golang/protobuf/protoc-gen-go@347cf4a; \
  $(FLAG_MODULE) $(GOGET) golang.org/x/tools/cmd/goimports

version:
	echo "package output" > output/version.go
	echo "" >> output/version.go
	echo "const Version = \"$(VERSION)"\" >> output/version.go
	echo "const Build = \"$(DATE)\"" >> output/version.go

peg:
	cd grammar; \
	peg grammar.peg; \
	goimports -w grammar.peg.go; \

asset:
	cd fs; \
	$(FLAGS_SHARED) go run -tags=dev assets_gen.go

build: version peg pb asset
	cd gnparser; \
	$(GOCLEAN); \
	$(FLAGS_SHARED) $(GOBUILD)

install: version peg pb asset
	cd gnparser; \
	$(GOCLEAN); \
	$(FLAGS_SHARED) $(GOINSTALL)

release: version peg pb asset dockerhub
	cd gnparser; \
	$(GOCLEAN); \
	$(FLAGS_LINUX) $(GOBUILD); \
	tar zcf /tmp/gnparser-$(VER)-linux.tar.gz gnparser; \
	$(GOCLEAN); \
	$(FLAGS_MAC) $(GOBUILD); \
	tar zcf /tmp/gnparser-$(VER)-mac.tar.gz gnparser; \
	$(GOCLEAN); \
	$(FLAGS_WIN) $(GOBUILD); \
	zip -9 /tmp/gnparser-$(VER)-win-64.zip gnparser.exe; \
	$(GOCLEAN);

.PHONY:pb
pb:
	cd pb; \
	protoc -I . ./gnparser.proto --go_out=plugins=grpc:.;

docker: build
	docker build -t gnames/gognparser:latest -t gnames/gognparser:$(VERSION) .; \
	cd gnparser; \
	$(GOCLEAN);

dockerhub: docker
	docker push gnames/gognparser; \
	docker push gnames/gognparser:$(VERSION)
