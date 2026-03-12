package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	_ "os"
	"time"

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
	if r.Method == http.MethodPost {
		ip := r.RemoteAddr
		if !getLoginLimiter(ip).Allow() {
			http.Error(w, "too many login attempts", 429)
			return
		}
		cookie, err := r.Cookie("csrf_token")
		formToken := r.FormValue("csrf_token")
		if err != nil || cookie.Value != formToken {
			http.Error(w, "invalid csrf", 403)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		var hash string
		var userID int
		err = db.QueryRow(`SELECT id, password_hash 
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
	limit := 6
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
		Site_Title string
	}{
		Posts:      posts,
		Miscs:      miscs,
		Page:       page,
		Site_Title: site_title,
	}

	renderTemplate(w, r, "dashboard.html", data)
}

func indexPage(w http.ResponseWriter, r *http.Request) {

	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
	}
	if page < 1 {
		page = 1
	}

	limit := 6
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
		Title         string
		Site_Title    string
		Site_Subtitle string
	}{
		Posts:         posts,
		Page:          page,
		Site_Title:    site_title,
		Site_Subtitle: site_subtitle,
	}
	renderTemplate(w, r, "index.html", data)
}

func renderTemplate(w http.ResponseWriter, r *http.Request, filename string, data interface{}) {

	themePath := "themes/" + theme + "/templates/"

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
		SameSite: http.SameSiteStrictMode,
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
