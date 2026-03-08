package main

import (
	"fmt"
    "log"
	"github.com/yuin/goldmark"
	 "github.com/yuin/goldmark/renderer/html"
	_ "modernc.org/sqlite"
	"net/http"
	"strings"
    "database/sql"
)

func Page(option int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			p      Post
			slug   string
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
			if option == 2 {
				miscIndex(w, r)
				return
			} else if option == 1 {
                blogIndex(w, r)
                return
                }
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
				"Slug":  slug,
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
		title := r.FormValue("page_title")
		content := r.FormValue("content")
		status := r.FormValue("status")
		pass := r.FormValue("page_password")
		xtype := r.FormValue("type")
		date := r.FormValue("date")
		slug := slugify(title)

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
    userID, valid := checkSession(w,r)

    if !valid {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }

    slug := strings.TrimPrefix(r.URL.Path, "/edit/")
    if slug == "" {
        http.NotFound(w, r)
        return
    }

    var username string
    err := db.QueryRow("SELECT username FROM users WHERE id = ?",
        userID).Scan(&username)

    if err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "user not found", http.StatusUnauthorized)
        } else {
            http.Error(w, "failed to get username", http.StatusInternalServerError)
        }
        return
    }

    var p Post
    err = db.QueryRow(`SELECT id, date, author, title, slug, content, status,
    type, pass FROM posts WHERE slug=?`, slug).Scan(&p.ID, &p.Date, &p.Author,
    &p.Title, &p.Slug, &p.Content, &p.Status, &p.Type, &p.Pass)

    if err != nil {
        if err == sql.ErrNoRows {
            http.NotFound(w, r)
        } else {
            http.Error(w, "database error lul", http.StatusInternalServerError)
        }
        return
    }

    if p.Author != username {
        http.Error(w, "who?", http.StatusInternalServerError)
        return
    }
    if r.Method == http.MethodPost {
		title := r.FormValue("page_title")
		content := r.FormValue("content")
		status := r.FormValue("status")
		date := r.FormValue("date")
		xtype := r.FormValue("type")
		currentPassword := r.FormValue("current_password")
		newPassword := r.FormValue("page_password")
		passwordToggle := r.FormValue("password_toggle")

		newSlug := slugify(title)

        if p.Pass != "" && !checkPasswordHash(p.Pass, currentPassword) {
			renderTemplate(w, r, "editpage.html", map[string]interface{}{
				"ID":      p.ID,
				"Date":    p.Date,
				"Author":  p.Author,
				"Title":   p.Title,
				"Slug":    p.Slug,
				"Content": p.Content,
				"Status":  p.Status,
				"Type":    p.Type,
				"HasPass": p.Pass != "",
				"Error":   "Current password is incorrect",
			})
			return
		}

        var passhash string
		if passwordToggle == "on" && newPassword != "" {
			passhash, err = hashPassword(newPassword)
			if err != nil {
				http.Error(w, "failed to hash password",
                http.StatusInternalServerError)
				return
			}
		} else if passwordToggle != "on" {
			passhash = ""
		} else {
			passhash = p.Pass
		}

        _, err = db.Exec(`UPDATE posts SET date=?, title=?, slug=?, content=?,
        status=?, type=?, pass=? WHERE id=?`,
			date, title, newSlug, content, status, xtype, passhash, p.ID)
		
		if err != nil {
			log.Println("DB update error:", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}
        prefix := "/"
        if xtype == "blog" {
            prefix = "/blog/"
        } else if xtype == "misc" {
            prefix = "/misc/"
        }

        http.Redirect(w, r, prefix+newSlug, http.StatusSeeOther)
        return
    }
    renderTemplate(w, r, "editpage.html", map[string]interface{}{
		"ID":      p.ID,
		"Date":    p.Date,
		"Author":  p.Author,
		"Title":   p.Title,
		"Slug":    p.Slug,
		"Content": p.Content,
		"Status":  p.Status,
		"Type":    p.Type,
		"HasPass": p.Pass != "",
	})
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
    return func(w http.ResponseWriter, r *http.Request) {
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

        pw := r.FormValue("page_password")
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

func miscIndex(w http.ResponseWriter, r *http.Request) {

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
    status FROM posts WHERE type='misc' AND status='post'
    ORDER BY date DESC, id DESC LIMIT ? OFFSET ?`,
    limit, offset)

    if err != nil {
        http.Error(w, "db error", 500)
        return
    }
    defer rows.Close()
    var miscs []Post
    for rows.Next() {
        var m Post
        rows.Scan(&m.ID, &m.Date, &m.Author, &m.Type, &m.Title, &m.Slug, &m.Content,
        &m.Status)
        miscs = append(miscs, m)
    }

    data := struct {
        Posts []Post
        Page  int
        Title string
        Site_Title string
    }{
        Posts: miscs,
        Page:  page,
        Site_Title: site_title,
    }
    renderTemplate(w, r, "misc.html", data)
}

func blogIndex(w http.ResponseWriter, r *http.Request) {

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
        Posts []Post
        Page  int
        Title string
        Site_Title string
    }{
        Posts: posts,
        Page:  page,
        Site_Title: site_title,
    }
    renderTemplate(w, r, "blog.html", data)
}
