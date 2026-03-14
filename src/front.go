package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"golang.org/x/time/rate"
	"sync"
)

var tmplFuncs = template.FuncMap{
	"safeHTML": func(s string) template.HTML {
		return template.HTML(s)
	},
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
	},
}

var (
	loginLimiters = make(map[string]*rate.Limiter)
	loginMu       sync.Mutex
)

var front_page_type string = "blog"
var front_page_custom string = ""

func getLoginLimiter(ip string) *rate.Limiter {
	loginMu.Lock()
	defer loginMu.Unlock()

	limiter, exists := loginLimiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Minute/5), 5)
		loginLimiters[ip] = limiter
	}
	return limiter
}
func loginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	if r.Method == http.MethodPost {
		ip := r.RemoteAddr
		if !getLoginLimiter(ip).Allow() {
			http.Error(w, "too many login attempts", 429)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		var hash string
		var userID int
		err := db.QueryRow(`SELECT id, password_hash 
		FROM users WHERE username=?`,
			username).Scan(&userID, &hash)
		if err != nil {
			http.Error(w, "invalid username or password", 401)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hash),
			[]byte(password)) != nil {
			http.Error(w, "invalid username or password", 401)
			return
		}
		createSession(w, userID)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	renderTemplate(w, r, "login.html", nil)
}

func signupPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	if r.Method == http.MethodPost {
		cookie, err := r.Cookie("csrf_token")
		if err != nil || cookie.Value != r.FormValue("csrf_token") {
			http.Error(w, "invalid csrf", 403)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		hash, err := bcrypt.GenerateFromPassword([]byte(password),
			bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "hash error", 500)
			return
		}
		_, err = db.Exec(`INSERT INTO users(username, password_hash)
		VALUES(?, ?)`,
			username, hash)
		if err != nil {
			http.Error(w, "username already used", 400)
			return
		}
		var userID int
		db.QueryRow(`SELECT id 
		FROM users WHERE username=?`, username).Scan(&userID)
		createSession(w, userID)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	renderTemplate(w, r, "signup.html", nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		db.Exec("DELETE FROM sessions WHERE id=?", cookie.Value)
		http.SetCookie(w, &http.Cookie{
			Name:    "session",
			Value:   "",
			Path:    "/",
			Expires: time.Unix(0, 0),
		})
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func dashboardPage(w http.ResponseWriter, r *http.Request) {
	author := getUsername(w, r)

	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
	}
	if page < 1 {
		page = 1
	}
	limit := pagination_limit
	offset := (page - 1) * limit

	rows, err := db.Query(`SELECT id, date, author, type, title, slug, content, status
        FROM posts WHERE author=? AND type='blog' ORDER BY id DESC LIMIT ? OFFSET ?`,
		author, limit, offset)
	if err != nil {
		http.Error(w, "db error rows", 500)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Slug,
			&p.Content, &p.Status)
		if p.Status == "draft" {
			p.Title += " (draft)"
		}
		posts = append(posts, p)
	}

	miscrows, err := db.Query(`SELECT id, date, author, type, title, slug, 
    content, status FROM posts WHERE author=? AND type='misc' ORDER BY id DESC 
    LIMIT ? OFFSET ?`, author, limit, offset)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer miscrows.Close()

	var miscs []Post
	for miscrows.Next() {
		var m Post
		miscrows.Scan(&m.ID, &m.Date, &m.Author, &m.Type, &m.Title, &m.Slug,
			&m.Content, &m.Status)
		if m.Status == "draft" {
			m.Title += " (draft)"
		}
		miscs = append(miscs, m)
	}

	data := struct {
		Posts      []Post
		Miscs      []Post
		Page       int
		Limit      int
		Site_Title string
	}{
		Posts:      posts,
		Miscs:      miscs,
		Page:       page,
		Limit:      limit,
		Site_Title: site_title,
	}

	renderTemplate(w, r, "dashboard.html", data)
}

func indexPage(w http.ResponseWriter, r *http.Request) {
	if front_page_type == "custom" && front_page_custom != "" {
		renderCustomFrontPage(w, r)
		return
	}

	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
	}
	if page < 1 {
		page = 1
	}

	limit := pagination_limit
	offset := (page - 1) * limit

	rows, err := db.Query(`SELECT id, date, author, type, title, slug, content,
	status FROM posts WHERE type='blog' AND status='post'
	ORDER BY date DESC, id DESC LIMIT ? OFFSET ?`,
		limit, offset)

	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Slug, &p.Content,
			&p.Status)
		posts = append(posts, p)
	}

	data := struct {
		Posts         []Post
		Page          int
		Limit         int
		Title         string
		Site_Title    string
		Site_Subtitle string
	}{
		Posts:         posts,
		Page:          page,
		Limit:         limit,
		Site_Title:    site_title,
		Site_Subtitle: site_subtitle,
	}
	renderTemplate(w, r, "index.html", data)
}

func renderCustomFrontPage(w http.ResponseWriter, r *http.Request) {
	gm := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	var sb strings.Builder
	gm.Convert([]byte(front_page_custom), &sb)

	data := struct {
		HTML          string
		Site_Title    string
		Site_Subtitle string
	}{
		HTML:          sb.String(),
		Site_Title:    site_title,
		Site_Subtitle: site_subtitle,
	}
	renderTemplate(w, r, "custom_front.html", data)
}

func renderTemplate(w http.ResponseWriter, r *http.Request, filename string, data interface{}) {
	themePath := filepath.Join(themeDir, theme, "templates") + string(filepath.Separator)

	tmpl, err := template.New(filename).Funcs(tmplFuncs).ParseFiles(themePath + filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	head, err := template.New("head.html").Funcs(tmplFuncs).ParseFiles(themePath + "head.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	header, err := template.ParseFiles(themePath + "header.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	footer, err := template.ParseFiles(themePath + "footer.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, valid := checkSession(w, r)
	username := ""
	if valid {
		username = getUsername(w, r)
	}

	csrfToken := generateCSRFToken()

	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	commonData := map[string]interface{}{
		"CSRF":          csrfToken,
		"Logged":        valid,
		"Username":      username,
		"Site_Title":    site_title,
		"Site_Subtitle": site_subtitle,
	}

	var templateData interface{}
	if data == nil {
		templateData = commonData
	} else if m, ok := data.(map[string]interface{}); ok {
		for k, v := range commonData {
			m[k] = v
		}
		templateData = m
	} else {
		templateData = data
	}

	headerData := map[string]interface{}{
		"CSRF":          csrfToken,
		"Logged":        valid,
		"Username":      username,
		"Site_Title":    site_title,
		"Site_Subtitle": site_subtitle,
	}

	if err := head.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := header.Execute(w, headerData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, templateData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := footer.Execute(w, headerData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func generateCSRFToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func settingsPage(w http.ResponseWriter, r *http.Request) {
	userID, valid := checkSession(w, r)
	if !valid {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var username string
	var isAdmin bool
	var profileName, profileDesc, profileImg string
	var userPagination, userRssLimit int
	var userEnableRSS bool
	err := db.QueryRow("SELECT username, is_admin, profile_name, profile_description, profile_image, pagination_limit, enable_rss, rss_limit FROM users WHERE id = ?", userID).Scan(&username, &isAdmin, &profileName, &profileDesc, &profileImg, &userPagination, &userEnableRSS, &userRssLimit)
	if err != nil {
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}

	getSettingsData := func(errMsg, successMsg string) map[string]interface{} {
		return map[string]interface{}{
			"Username":           username,
			"IsAdmin":            isAdmin,
			"ProfileName":        profileName,
			"ProfileDescription": profileDesc,
			"ProfileImage":       profileImg,
			"UserPagination":     userPagination,
			"UserEnableRSS":      userEnableRSS,
			"UserRssLimit":       userRssLimit,
			"PaginationLimit":    pagination_limit,
			"FrontPageType":      front_page_type,
			"FrontPageCustom":    front_page_custom,
			"SiteTitle":          site_title,
			"SiteSubtitle":       site_subtitle,
			"SiteURL":            site_url,
			"Error":              errMsg,
			"Success":            successMsg,
		}
	}

	if r.Method == http.MethodPost {
		cookie, err := r.Cookie("csrf_token")
		if err != nil || cookie.Value != r.FormValue("csrf_token") {
			http.Error(w, "invalid csrf", 403)
			return
		}

		action := r.FormValue("action")

		if action == "profile" {
			name := r.FormValue("profile_name")
			desc := r.FormValue("profile_description")
			img := r.FormValue("profile_image")

			_, err := db.Exec("UPDATE users SET profile_name = ?, profile_description = ?, profile_image = ? WHERE id = ?", name, desc, img, userID)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			profileName = name
			profileDesc = desc
			profileImg = img

			renderTemplate(w, r, "settings.html", getSettingsData("", "profile updated"))
			return
		}

		if action == "preferences" {
			pagLimit := r.FormValue("pagination_limit")
			rssLimit := r.FormValue("rss_limit")
			enableRSS := r.FormValue("enable_rss") == "1"

			var pagInt, rssInt int
			fmt.Sscanf(pagLimit, "%d", &pagInt)
			fmt.Sscanf(rssLimit, "%d", &rssInt)

			if pagInt < 1 || pagInt > 100 {
				pagInt = 10
			}
			if rssInt < 1 || rssInt > 100 {
				rssInt = 50
			}

			enableInt := 0
			if enableRSS {
				enableInt = 1
			}

			_, err := db.Exec("UPDATE users SET pagination_limit = ?, enable_rss = ?, rss_limit = ? WHERE id = ?", pagInt, enableInt, rssInt, userID)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			userPagination = pagInt
			userRssLimit = rssInt
			userEnableRSS = enableRSS

			renderTemplate(w, r, "settings.html", getSettingsData("", "preferences updated"))
			return
		}

		if action == "change_password" {
			currentPassword := r.FormValue("current_password")
			newPassword := r.FormValue("new_password")
			confirmPassword := r.FormValue("confirm_password")

			if newPassword != confirmPassword {
				renderTemplate(w, r, "settings.html", getSettingsData("new password and confirmation don't match", ""))
				return
			}

			var hash string
			err = db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hash)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)) != nil {
				renderTemplate(w, r, "settings.html", getSettingsData("current password is incorrect", ""))
				return
			}

			newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "hash error", http.StatusInternalServerError)
				return
			}

			_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", newHash, userID)
			if err != nil {
				http.Error(w, "db update error", http.StatusInternalServerError)
				return
			}

			renderTemplate(w, r, "settings.html", getSettingsData("", "password changed successfully"))
			return
		}

		if !isAdmin {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}

		if action == "site" {
			title := r.FormValue("site_title")
			subtitle := r.FormValue("site_subtitle")
			url := r.FormValue("site_url")

			err := saveSetting("site_title", title)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			err = saveSetting("site_subtitle", subtitle)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			err = saveSetting("site_url", url)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			site_title = title
			site_subtitle = subtitle
			site_url = url

			renderTemplate(w, r, "settings.html", getSettingsData("", "site settings updated"))
			return
		}

		if action == "frontpage" {
			fpt := r.FormValue("front_page_type")
			fpc := r.FormValue("front_page_custom")

			if fpt != "blog" && fpt != "custom" {
				fpt = "blog"
			}

			err := saveSetting("front_page_type", fpt)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			err = saveSetting("front_page_custom", fpc)
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}

			front_page_type = fpt
			front_page_custom = fpc

			renderTemplate(w, r, "settings.html", getSettingsData("", "front page settings updated"))
			return
		}
	}

	renderTemplate(w, r, "settings.html", getSettingsData("", ""))
}
