package main

import (
	"strings"
	"net/http"
	"time"
	"fmt"
	_ "modernc.org/sqlite"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

func Page(option int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			p Post
			slug string
			column string
		)
		gm := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
		if option == 1 {
			slug = strings.TrimPrefix(r.URL.Path, "/blog/")
			column = "blog"
		} else if option == 2 {
			slug = strings.TrimPrefix(r.URL.Path, "/misc/")
			column = "misc"
		} else {
			http.NotFound(w, r)
			return
		}

		if slug == "" {
			http.NotFound(w, r)
			return
		}

		_, valid := checkSession(w, r)
		var err error

		if valid {
			err = db.QueryRow(`SELECT id, date, author, type, title, content, status 
			FROM posts WHERE slug=? AND type=?`, slug, column).
			Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Content, &p.Status)
		} else {
			err = db.QueryRow(`SELECT id, date, author, type, title, content, status, pass 
			FROM posts 
			WHERE slug=? AND status='post' AND type=?`, slug, column).
			Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Content, &p.Status, &p.Pass)
		}

		if err != nil {
			http.NotFound(w, r)
			return
		}

		if p.Pass != "" && !valid && !isUnlocked(r, slug) {
			renderTemplate(w, r, "page_password.html", map[string]interface{}{
				"Slug": slug,
				"Title": p.Title})
				return
			}
			var sb strings.Builder
			if err := gm.Convert([]byte(p.Content), &sb); err == nil {
				p.HTML = sb.String()
			}

			renderTemplate(w, r, "page.html", p)
		}
	}


	func newPage(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			title := r.FormValue("title")
			content := r.FormValue("content")
			status := r.FormValue("status")
			pass := r.FormValue("pass")
			xtype := r.FormValue("type")

			slug := slugify(title)
			date := time.Now().Format(time.RFC3339)

			userID, valid := checkSession(w, r)
			if !valid {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			var userName string
			err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&userName)
			if err != nil {
				http.Error(w, "failed to get username", http.StatusInternalServerError)
				return
			}

			passhash, err := hashPassword(pass)
			if err != nil {
				http.Error(w, "failed to hash password", 
				http.StatusInternalServerError)
			}
			_, err = db.Exec(`INSERT INTO 
			posts(date, author, type, title, slug, content, status, pass)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,

			date, userName, xtype, title, slug, content, status, passhash)
			if err != nil {
				http.Error(w, "db insert error", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		renderTemplate(w, r, "newpage.html", nil)
	}

	func editPage(w http.ResponseWriter, r *http.Request) {
		userID, valid := checkSession(w, r)
		if !valid {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var username string
		err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			http.Error(w, "failed to get username", http.StatusInternalServerError)
			return
		}

		var p Post
		slug := strings.TrimPrefix(r.URL.Path, "/edit/")
		if slug == "" {
			http.NotFound(w, r)
			return
		}

		err = db.QueryRow(`SELECT author, status FROM posts WHERE slug=?`, slug).Scan(&p.Author, &p.Status)
		if err != nil || p.Author != username {
			http.Error(w, ":(", http.StatusForbidden)
			return
		}

		if r.Method == http.MethodPost {
			title := r.FormValue("title")
			content := r.FormValue("content")
			status := r.FormValue("status")
			// xtype := r.FormValue("type")

			newSlug := slugify(title)

			_, err := db.Exec(`UPDATE posts SET title=?, slug=?, content=?, status=? WHERE slug=?`,
			title, newSlug, content, status, slug)
			if err != nil {
				http.Error(w, "db update error", 500)
				return
			}

			http.Redirect(w, r, "/post/"+newSlug, http.StatusSeeOther)
			return
		}

		err = db.QueryRow(`SELECT id, date, author, title, slug, content FROM posts WHERE slug=?`, slug).
		Scan(&p.ID, &p.Date, &p.Author, &p.Title, &p.Slug, &p.Content)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		renderTemplate(w, r, "editpage.html", p)
	}

	func deletePage(w http.ResponseWriter, r *http.Request) {
		userID, valid := checkSession(w, r)
		if !valid {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var username string
		err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			http.Error(w, "failed to get username", http.StatusInternalServerError)
			return
		}

		slug := strings.TrimPrefix(r.URL.Path, "/delete/")
		if slug == "" {
			http.NotFound(w, r)
			return
		}

		var author string
		err = db.QueryRow(`SELECT author FROM posts WHERE slug=?`, slug).Scan(&author)
		if err != nil || author != username {
			http.Error(w, ":(", http.StatusForbidden)
			return
		}

		_, err = db.Exec(`DELETE FROM posts WHERE slug=?`, slug)
		if err != nil {
			http.Error(w, "db delete error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}


	func pageUnlock(option int) http.HandlerFunc {
		optionx := option
		return func (w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			fmt.Println(optionx)
			var slug string
			if optionx == 1 {
				slug = strings.TrimPrefix(r.URL.Path, "/blog/")
			} else if optionx == 2 {
				slug = strings.TrimPrefix(r.URL.Path, "/misc/")
			}
			slug = strings.TrimSuffix(slug, "/unlock")

			pw := r.FormValue("password")
			var hash string
			err := db.QueryRow(`SELECT pass FROM posts WHERE slug = ?`, slug).Scan(&hash)
			if err != nil || hash == "" {
				if optionx == 1 {
				http.Redirect(w, r, "/blog/"+slug, http.StatusSeeOther)
			} else if optionx == 2 {	
				http.Redirect(w, r, "/misc/"+slug, http.StatusSeeOther)
			}
				return
			}

			if !checkPasswordHash(hash, pw) {
				renderTemplate(w, r, "page_password.html", map[string]interface{}{
					"Slug": slug, "Error": ":(", "Option": optionx,
				})
				return
			}

			setUnlockedCookie(w, slug)
			if optionx == 1 {
				http.Redirect(w, r, "/blog/"+slug, http.StatusSeeOther)
			} else if optionx == 2 {
				http.Redirect(w, r, "/misc/"+slug, http.StatusSeeOther)
			}
		}
	}
