build:
	mkdir -p dist
	go build -o dist/oa ./cmd/oa

install:
	go install ./cmd/oa
