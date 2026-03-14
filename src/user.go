package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/crypto/bcrypt"
)

type UserProfile struct {
	Name        string
	Description string
	Image       string
}

func getUserProfile(username string) UserProfile {
	var profile UserProfile
	db.QueryRow("SELECT profile_name, profile_description, profile_image FROM users WHERE username = ?", username).Scan(&profile.Name, &profile.Description, &profile.Image)
	return profile
}

func usersPage(w http.ResponseWriter, r *http.Request) {
	userID, valid := checkSession(w, r)
	if !valid {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var isAdmin bool
	db.QueryRow("SELECT is_admin FROM users WHERE id = ?", userID).Scan(&isAdmin)

	if r.Method == http.MethodPost {
		if !isAdmin {
			http.Error(w, "admin only", http.StatusForbidden)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		isAdminCheckbox := r.FormValue("is_admin") == "1"

		if username == "" || password == "" {
			renderTemplate(w, r, "users.html", map[string]interface{}{
				"Error":   "username and password required",
				"IsAdmin": isAdmin,
			})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "hash error", http.StatusInternalServerError)
			return
		}

		isAdminInt := 0
		if isAdminCheckbox {
			isAdminInt = 1
		}

		_, err = db.Exec(`INSERT INTO users(username, password_hash, is_admin) VALUES(?, ?, ?)`, username, string(hash), isAdminInt)
		if err != nil {
			renderTemplate(w, r, "users.html", map[string]interface{}{
				"Error":   "username already exists",
				"IsAdmin": isAdmin,
			})
			return
		}

		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	rows, err := db.Query(`SELECT id, username, is_admin FROM users ORDER BY id`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type User struct {
		ID       int
		Username string
		IsAdmin  bool
	}
	var users []User
	for rows.Next() {
		var u User
		var isAdminInt int
		rows.Scan(&u.ID, &u.Username, &isAdminInt)
		u.IsAdmin = isAdminInt == 1
		users = append(users, u)
	}

	var currentUser string
	db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&currentUser)

	renderTemplate(w, r, "users.html", map[string]interface{}{
		"Users":    users,
		"Username": currentUser,
		"IsAdmin":  isAdmin,
	})
}

func toggleAdminHandler(w http.ResponseWriter, r *http.Request) {
	userID, valid := checkSession(w, r)
	if !valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var isAdmin bool
	db.QueryRow("SELECT is_admin FROM users WHERE id = ?", userID).Scan(&isAdmin)

	if !isAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	var currentUser string
	db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&currentUser)

	if username == currentUser {
		http.Error(w, "cannot toggle your own admin status", http.StatusBadRequest)
		return
	}

	db.Exec("UPDATE users SET is_admin = CASE WHEN is_admin = 1 THEN 0 ELSE 1 END WHERE username = ?", username)

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, valid := checkSession(w, r)
	if !valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var isAdmin bool
	db.QueryRow("SELECT is_admin FROM users WHERE id = ?", userID).Scan(&isAdmin)

	if !isAdmin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count <= 1 {
		http.Error(w, "cannot delete last user", http.StatusBadRequest)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	var currentUser string
	db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&currentUser)

	if username == currentUser {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		http.Error(w, "delete error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func userPage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	username := parts[0]

	var exists int
	err := db.QueryRow("SELECT 1 FROM users WHERE username = ?", username).Scan(&exists)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 || parts[1] == "" {
		blogIndexUser(w, r, username)
		return
	}

	subPath := parts[1]

	if strings.HasPrefix(subPath, "blog/") {
		slug := strings.TrimPrefix(subPath, "blog/")
		if slug == "" {
			blogIndexUser(w, r, username)
			return
		}
		userBlogPost(w, r, username, slug)
		return
	}

	if strings.HasPrefix(subPath, "misc/") {
		slug := strings.TrimPrefix(subPath, "misc/")
		if slug == "" {
			miscIndexUser(w, r, username)
			return
		}
		userMiscPost(w, r, username, slug)
		return
	}

	if subPath == "rss" {
		userRSSFeed(w, r, username)
		return
	}

	http.NotFound(w, r)
}

func userRSSFeed(w http.ResponseWriter, r *http.Request, username string) {
	var enableRSS int
	var rssLimit int
	err := db.QueryRow("SELECT enable_rss, rss_limit FROM users WHERE username = ?", username).Scan(&enableRSS, &rssLimit)
	if err != nil || enableRSS == 0 {
		http.NotFound(w, r)
		return
	}

	var profileName, profileDesc string
	db.QueryRow("SELECT profile_name, profile_description FROM users WHERE username = ?", username).Scan(&profileName, &profileDesc)

	if rssLimit < 1 || rssLimit > 100 {
		rssLimit = 50
	}

	rows, err := db.Query(`SELECT date, title, slug, content FROM posts 
		WHERE author=? AND type='blog' AND status='post' 
		ORDER BY date DESC LIMIT ?`, username, rssLimit)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	type Item struct {
		Date    string
		Title   string
		Slug    string
		Content string
	}
	var items []Item
	for rows.Next() {
		var it Item
		rows.Scan(&it.Date, &it.Title, &it.Slug, &it.Content)
		items = append(items, it)
	}

	gm := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<rss version="2.0">`)
	sb.WriteString("<channel>")
	sb.WriteString(fmt.Sprintf("<title>%s</title>", escapeXML(profileName)))
	sb.WriteString(fmt.Sprintf("<link>%s/%s</link>", escapeXML(site_url), username))
	sb.WriteString(fmt.Sprintf("<description>%s</description>", escapeXML(profileDesc)))
	sb.WriteString("<language>en-us</language>")

	for _, it := range items {
		var contentHTML strings.Builder
		gm.Convert([]byte(it.Content), &contentHTML)

		sb.WriteString("<item>")
		sb.WriteString(fmt.Sprintf("<title>%s</title>", escapeXML(it.Title)))
		sb.WriteString(fmt.Sprintf("<link>%s/%s/blog/%s</link>", escapeXML(site_url), username, it.Slug))
		sb.WriteString(fmt.Sprintf("<pubDate>%s</pubDate>", formatDate(it.Date)))
		sb.WriteString(fmt.Sprintf("<description>%s</description>", escapeXML(contentHTML.String())))
		sb.WriteString(fmt.Sprintf("<guid>%s/%s/blog/%s</guid>", escapeXML(site_url), username, it.Slug))
		sb.WriteString("</item>")
	}

	sb.WriteString("</channel>")
	sb.WriteString("</rss>")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(sb.String()))
}

func blogIndexUser(w http.ResponseWriter, r *http.Request, username string) {
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
		FROM posts WHERE author=? AND type='blog' AND status='post' 
		ORDER BY date DESC, id DESC LIMIT ? OFFSET ?`,
		username, limit, offset)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Slug, &p.Content, &p.Status)
		posts = append(posts, p)
	}

	profile := getUserProfile(username)

	data := struct {
		Posts               []Post
		Page                int
		Limit               int
		Username            string
		Site_Title          string
		Site_Subtitle       string
		Profile_Name        string
		Profile_Description string
		Profile_Image       string
	}{
		Posts:               posts,
		Page:                page,
		Limit:               limit,
		Username:            username,
		Site_Title:          site_title,
		Site_Subtitle:       site_subtitle,
		Profile_Name:        profile.Name,
		Profile_Description: profile.Description,
		Profile_Image:       profile.Image,
	}
	renderTemplate(w, r, "user_blog.html", data)
}

func miscIndexUser(w http.ResponseWriter, r *http.Request, username string) {
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
		FROM posts WHERE author=? AND type='misc' AND status='post' 
		ORDER BY date DESC, id DESC LIMIT ? OFFSET ?`,
		username, limit, offset)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Slug, &p.Content, &p.Status)
		posts = append(posts, p)
	}

	profile := getUserProfile(username)

	data := struct {
		Posts               []Post
		Page                int
		Limit               int
		Username            string
		Site_Title          string
		Site_Subtitle       string
		Profile_Name        string
		Profile_Description string
		Profile_Image       string
	}{
		Posts:               posts,
		Page:                page,
		Limit:               limit,
		Username:            username,
		Site_Title:          site_title,
		Site_Subtitle:       site_subtitle,
		Profile_Name:        profile.Name,
		Profile_Description: profile.Description,
		Profile_Image:       profile.Image,
	}
	renderTemplate(w, r, "user_misc.html", data)
}

func userBlogPost(w http.ResponseWriter, r *http.Request, username, slug string) {
	var p Post
	_, valid := checkSession(w, r)
	var err error

	if valid {
		err = db.QueryRow(`SELECT id, date, author, type, title, content, status, thumbnail, youtube 
			FROM posts WHERE slug=? AND author=? AND type='blog'`, slug, username).
			Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Content, &p.Status, &p.Thumbnail, &p.Youtube)
	} else {
		err = db.QueryRow(`SELECT id, date, author, type, title, content, status, pass, thumbnail, youtube 
			FROM posts WHERE slug=? AND author=? AND type='blog' AND status='post'`, slug, username).
			Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Content, &p.Status, &p.Pass, &p.Thumbnail, &p.Youtube)
	}

	if err != nil {
		http.NotFound(w, r)
		return
	}

	if p.Pass != "" && !valid && !isUnlocked(r, slug) {
		renderTemplate(w, r, "page_password.html", map[string]interface{}{
			"Slug":  slug,
			"Title": p.Title,
		})
		return
	}

	gm := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	var sb strings.Builder
	if err := gm.Convert([]byte(p.Content), &sb); err == nil {
		p.HTML = sb.String()
	}

	profile := getUserProfile(username)

	renderTemplate(w, r, "page.html", map[string]interface{}{
		"ID":                  p.ID,
		"Date":                p.Date,
		"Author":              p.Author,
		"Type":                p.Type,
		"Title":               p.Title,
		"Slug":                p.Slug,
		"Content":             p.Content,
		"Status":              p.Status,
		"Pass":                p.Pass,
		"Thumbnail":           p.Thumbnail,
		"Youtube":             p.Youtube,
		"HTML":                p.HTML,
		"Profile_Name":        profile.Name,
		"Profile_Description": profile.Description,
		"Profile_Image":       profile.Image,
	})
}

func userMiscPost(w http.ResponseWriter, r *http.Request, username, slug string) {
	var p Post
	_, valid := checkSession(w, r)
	var err error

	if valid {
		err = db.QueryRow(`SELECT id, date, author, type, title, content, status, thumbnail, youtube 
			FROM posts WHERE slug=? AND author=? AND type='misc'`, slug, username).
			Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Content, &p.Status, &p.Thumbnail, &p.Youtube)
	} else {
		err = db.QueryRow(`SELECT id, date, author, type, title, content, status, pass, thumbnail, youtube 
			FROM posts WHERE slug=? AND author=? AND type='misc' AND status='post'`, slug, username).
			Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Content, &p.Status, &p.Pass, &p.Thumbnail, &p.Youtube)
	}

	if err != nil {
		http.NotFound(w, r)
		return
	}

	if p.Pass != "" && !valid && !isUnlocked(r, slug) {
		renderTemplate(w, r, "page_password.html", map[string]interface{}{
			"Slug":  slug,
			"Title": p.Title,
		})
		return
	}

	gm := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	var sb strings.Builder
	if err := gm.Convert([]byte(p.Content), &sb); err == nil {
		p.HTML = sb.String()
	}

	profile := getUserProfile(username)

	renderTemplate(w, r, "page.html", map[string]interface{}{
		"ID":                  p.ID,
		"Date":                p.Date,
		"Author":              p.Author,
		"Type":                p.Type,
		"Title":               p.Title,
		"Slug":                p.Slug,
		"Content":             p.Content,
		"Status":              p.Status,
		"Pass":                p.Pass,
		"Thumbnail":           p.Thumbnail,
		"Youtube":             p.Youtube,
		"HTML":                p.HTML,
		"Profile_Name":        profile.Name,
		"Profile_Description": profile.Description,
		"Profile_Image":       profile.Image,
	})
}
