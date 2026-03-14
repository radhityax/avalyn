package main

import (
	"os"
)

var version string = "beta"

var register_browser_mode int = 0

var site_title string = "avalyn"
var site_subtitle string = "an absurd web"

var theme string = "default"

var pagination_limit int = 10

var (
	dataDir  string
	themeDir string
	dbPath   string
)

func init() {
	dataDir = "/opt/avalyn"
	themeDir = "/opt/avalyn/themes"
	dbPath = "/opt/avalyn/avalyn.db"

	os.MkdirAll(dataDir, 0755)
}
