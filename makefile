src = src/avalyn.go src/cli.go src/front.go src/page.go src/config.go
target = avalyn
db = avalyn.db

base_flags = -modcacherw -tags "osusergo,netgo" -trimpath -buildvcs=false \
	     -ldflags="-s -w -buildid= -extldflags '-static -s -w'"

all:
	GOARCH=amd64 go build $(base_flags) \
	-o $(target) $(src)

phone:
	GOOS=linux GOARCH=arm64 go build $(base_flags) \
	     -o $(target) $(src)

init:
	go mod init avalyn
	go get modernc.org/sqlite
	go get golang.org/x/crypto/bcrypt
	go get github.com/yuin/goldmark

clean:
	rm -f $(db) $(target)

