package main

import (
	"os"
	"path/filepath"
)

var version string = "beta"

var register_browser_mode int = 0

var site_title string = "avalyn"
var site_subtitle string = "an absurd web"

var theme string = "default"

var pagination_limit int = 10

var (
	configDir string
	dataDir   string
	themeDir  string
	dbPath    string
	isSystemd bool
)

func init() {
	configDir, _ = os.Getwd()
	dataDir = configDir
	themeDir = filepath.Join(configDir, "themes")
	dbPath = filepath.Join(configDir, "avalyn.db")
	isSystemd = false
}

func setSystemPaths() {
	configDir = "/opt/avalyn"
	dataDir = "/opt/avalyn"
	themeDir = "/opt/avalyn/themes"
	dbPath = "/opt/avalyn/avalyn.db"
	isSystemd = true
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(themeDir, 0755)
}
