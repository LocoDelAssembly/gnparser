VERSION = $(shell git describe --tags)
VER = $(shell git describe --tags --abbrev=0)
DATE = $(shell date -u '+%Y-%m-%d_%H:%M:%S%Z')

FLAG_MODULE = GO111MODULE=on
FLAGS_SHARED = $(FLAG_MODULE) GOARCH=amd64
NO_C = CGO_ENABLED=0
FLAGS_LINUX = $(FLAGS_SHARED) GOOS=linux
FLAGS_MAC = $(FLAGS_SHARED) GOOS=darwin
FLAGS_WIN = $(FLAGS_SHARED) GOOS=windows
FLAGS_LD=-ldflags "-X github.com/gnames/gnparser/output.Build=${DATE} \
                  -X github.com/gnames/gnparser/output.Version=${VERSION}"
GOCMD = go
GOBUILD = $(GOCMD) build $(FLAGS_LD)
GOINSTALL = $(GOCMD) install $(FLAGS_LD)
GOCLEAN = $(GOCMD) clean
GOGET = $(GOCMD) get

all: install

test: deps install
	$(FLAG_MODULE) go test ./...

test-build: deps build

deps:
	$(FLAG_MODULE) $(GOGET) github.com/pointlander/peg@21bead84a59; \
	$(FLAG_MODULE) $(GOGET) github.com/shurcooL/vfsgen@6a9ea43; \
	$(FLAG_MODULE) $(GOGET) github.com/spf13/cobra/cobra@v0.0.6; \
	$(FLAG_MODULE) $(GOGET) github.com/onsi/ginkgo/ginkgo@v1.12.0; \
	$(FLAG_MODULE) $(GOGET) github.com/onsi/gomega@v1.9.0; \
  $(FLAG_MODULE) $(GOGET) github.com/golang/protobuf/protoc-gen-go@347cf4a; \
  $(FLAG_MODULE) $(GOGET) golang.org/x/tools/cmd/goimports

peg:
	cd grammar; \
	peg grammar.peg; \
	goimports -w grammar.peg.go; \

ragel:
	cd preprocess; \
	ragel -Z -G2 virus.rl; \
	ragel -Z -G2 noparse.rl

asset:
	cd fs; \
	$(FLAGS_SHARED) go run -tags=dev assets_gen.go

build: peg asset
	cd gnparser; \
	$(GOCLEAN); \
	$(FLAGS_SHARED) $(NO_C) $(GOBUILD)

install: peg asset
	cd gnparser; \
	$(GOCLEAN); \
	$(FLAGS_SHARED) $(NO_C) $(GOINSTALL)

release: peg asset dockerhub
	cd gnparser; \
	$(GOCLEAN); \
	$(FLAGS_LINUX) $(NO_C) $(GOBUILD); \
	tar zcf /tmp/gnparser-$(VER)-linux.tar.gz gnparser; \
	$(GOCLEAN); \
	$(FLAGS_MAC) $(NO_C) $(GOBUILD); \
	tar zcf /tmp/gnparser-$(VER)-mac.tar.gz gnparser; \
	$(GOCLEAN); \
	$(FLAGS_WIN) $(NO_C) $(GOBUILD); \
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

clib: peg pb asset
	cd binding; \
	$(GOBUILD) -buildmode=c-shared -o libgnparser.so;
