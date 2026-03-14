PREFIX ?= /usr/local
BINDIR = $(DESTDIR)$(PREFIX)/bin
OPTDIR = $(DESTDIR)/opt/avalyn
VARDIR = $(DESTDIR)/var/lib/avalyn
SYSCONFDIR = $(DESTDIR)/etc

TARGET = avalyn
DB = avalyn.db
SRC = $(wildcard src/*.go)

LDFLAGS = -s -w -buildid= -extldflags "-static"
GOFLAGS = -modcacherw -tags "osusergo,netgo" -trimpath -buildvcs=false

.PHONY: all phone init tidy clean install uninstall service-systemd service-sysvinit

all:
	CGO_ENABLED=0 GOARCH=amd64 go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(TARGET) $(SRC)

phone:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o $(TARGET) $(SRC)

init:
	go mod init avalyn || true
	go get modernc.org/sqlite \
		golang.org/x/crypto/bcrypt \
		github.com/yuin/goldmark \
		golang.org/x/time/rate

tidy:
	go mod tidy

clean:
	go clean -cache -testcache
	rm -f $(DB) $(TARGET)

install:
	install -Dm755 $(TARGET) $(BINDIR)/$(TARGET)
	install -dm755 $(OPTDIR)/themes
	cp -r themes/* $(OPTDIR)/themes/ 2>/dev/null || true
	install -dm755 $(VARDIR)

uninstall:
	rm -f $(BINDIR)/$(TARGET)
	rm -rf $(OPTDIR)
	rm -f $(SYSCONFDIR)/systemd/system/avalyn.service
	rm -f $(SYSCONFDIR)/init.d/avalyn
	[ -x /bin/systemctl ] && systemctl daemon-reload || true

service-systemd:
	install -dm755 $(SYSCONFDIR)/systemd/system
	printf "[Unit]\nDescription=Avalyn Web Server\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=$(PREFIX)/bin/avalyn -s\nWorkingDirectory=$(OPTDIR)\nRestart=always\nRestartSec=5\nStandardOutput=journal\nStandardError=journal\nSyslogIdentifier=avalyn\n\n[Install]\nWantedBy=multi-user.target\n" > $(SYSCONFDIR)/systemd/system/avalyn.service
	systemctl daemon-reload

service-sysvinit:
	printf '#!/bin/sh\n### BEGIN INIT INFO\n# Provides: avalyn\n# Required-Start: $$network $$remote_fs $$syslog\n# Required-Stop: $$network $$remote_fs $$syslog\n# Default-Start: 2 3 4 5\n# Default-Stop: 0 1 6\n# Short-Description: Avalyn\n### END INIT INFO\n\nDAEMON=$(BINDIR)/$(TARGET)\nOPTS="-s"\nDIR=$(OPTDIR)\nPID=/var/run/avalyn.pid\n\ncase "$$1" in\n  start) cd $$DIR && start-stop-daemon --start --background --make-pidfile --pidfile $$PID --exec $$DAEMON -- $$OPTS ;;\n  stop) start-stop-daemon --stop --pidfile $$PID && rm -f $$PID ;;\n  restart) $$0 stop; sleep 1; $$0 start ;;\n  status) [ -f $$PID ] && echo "Running" || echo "Stopped" ;;\n  *) echo "Usage: $$0 {start|stop|restart|status}"; exit 1 ;;\nesac\n' > $(SYSCONFDIR)/init.d/avalyn
	chmod +x $(SYSCONFDIR)/init.d/avalyn
