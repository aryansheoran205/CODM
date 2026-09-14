package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt" // 🔥 Password hash ke liye
)

// --- DATA MODELS ---
type UserProfile struct {
	GameName string `json:"game_name"`
	UID      string `json:"uid"`
}

type ChatMessage struct {
	Username string
	Message  string
}

type Event struct {
	Title     string
	ImagePath string
}

type Loadout struct {
	Title     string
	Category  string
	ImagePath string
}

type GunsmithPageData struct {
	CategoryName string
	SearchQuery  string
	Loadouts     []Loadout
}

// --- HELPER FUNCTIONS ---
func saveDataToFile(filename string, data interface{}) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error opening file:", err)
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		log.Println("Error encoding JSON:", err)
		return err
	}
	return nil
}

func render(w http.ResponseWriter, tmpl string, data interface{}) {
	files := []string{
		"./ui/html/base.layout.tmpl",
		"./ui/html/" + tmpl,
	}
	ts, err := template.ParseFiles(files...)
	if err != nil {
		log.Println("Parse error:", err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}
	err = ts.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Println("Execute error:", err.Error())
		http.Error(w, "Internal Server Error", 500)
	}
}

// --- PAGE HANDLERS ---
func home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rows, err := db.Query(`SELECT username, message FROM chats WHERE room = 'home' ORDER BY id DESC LIMIT 50`)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var chats []ChatMessage
	for rows.Next() {
		var c ChatMessage
		if err := rows.Scan(&c.Username, &c.Message); err == nil {
			chats = append(chats, c)
		}
	}
	render(w, "home.page.tmpl", chats)
}

func events(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT title, image_path FROM events ORDER BY id DESC`)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var eventList []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Title, &e.ImagePath); err == nil {
			eventList = append(eventList, e)
		}
	}
	render(w, "events.page.tmpl", eventList)
}

func reportBug(w http.ResponseWriter, r *http.Request)    { render(w, "report-bug.page.tmpl", nil) }
func reportPlayer(w http.ResponseWriter, r *http.Request) { render(w, "report-player.page.tmpl", nil) }
func suggestions(w http.ResponseWriter, r *http.Request)  { render(w, "suggestions.page.tmpl", nil) }

func profile(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		newUser := UserProfile{
			GameName: r.FormValue("game_name"),
			UID:      r.FormValue("uid"),
		}
		saveDataToFile("users_data.json", newUser)
		http.Redirect(w, r, "/profile", http.StatusFound)
		return
	}
	render(w, "profile.page.tmpl", nil)
}

func teamUp(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT username, message FROM chats WHERE room = 'teamup' ORDER BY id DESC LIMIT 50`)
	if err == nil {
		defer rows.Close()
		var chats []ChatMessage
		for rows.Next() {
			var c ChatMessage
			if err := rows.Scan(&c.Username, &c.Message); err == nil {
				chats = append(chats, c)
			}
		}
		render(w, "teamup.page.tmpl", chats)
		return
	}
	render(w, "teamup.page.tmpl", nil)
}

func chatSend(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		message := r.FormValue("message")
		room := r.FormValue("room")

		session, _ := store.Get(r, "codm-session")
		username := "Guest"
		if val, ok := session.Values["username"].(string); ok {
			username = val
		}

		db.Exec(`INSERT INTO chats (room, username, message) VALUES ($1, $2, $3)`, room, username, message)

		if room == "home" {
			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			http.Redirect(w, r, "/team-up", http.StatusFound)
		}
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func handleAddEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseMultipartForm(10 << 20)
		title := r.FormValue("title")
		file, header, err := r.FormFile("event_image")
		if err == nil {
			defer file.Close()
			saveDir := filepath.Join("ui", "static", "uploads", "events")
			os.MkdirAll(saveDir, os.ModePerm)
			filename := header.Filename
			dst, _ := os.Create(filepath.Join(saveDir, filename))
			defer dst.Close()
			io.Copy(dst, file)
			dbPath := "/static/uploads/events/" + filename

			db.Exec("INSERT INTO events (title, image_path) VALUES ($1, $2)", title, dbPath)
		}
	}
	http.Redirect(w, r, "/events", http.StatusSeeOther)
}

// 🔥 AUTHENTICATION HANDLERS (Login / Register activated)

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		email := r.FormValue("email")
		password := r.FormValue("password")

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Server Error", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec("INSERT INTO users (email, password_hash) VALUES ($1, $2)", email, string(hashedPassword))
		if err != nil {
			http.Error(w, "Email pehle se registered hai!", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	// GET request aane par form dikhana
	render(w, "register.page.tmpl", nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		email := r.FormValue("email")
		password := r.FormValue("password")

		var dbPasswordHash string
		err := db.QueryRow("SELECT password_hash FROM users WHERE email=$1", email).Scan(&dbPasswordHash)
		if err != nil {
			http.Error(w, "User nahi mila!", http.StatusUnauthorized)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(dbPasswordHash), []byte(password))
		if err != nil {
			http.Error(w, "Galat Password!", http.StatusUnauthorized)
			return
		}

		session, _ := store.Get(r, "codm-session")
		session.Values["authenticated"] = true
		session.Values["username"] = email
		session.Save(r, w)

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// GET request aane par form dikhana
	render(w, "login.page.tmpl", nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "codm-session")
	session.Values["authenticated"] = false
	session.Values["username"] = "Guest"
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- GUNSMITH HANDLERS ---
func gunsmith(w http.ResponseWriter, r *http.Request) {
	render(w, "gunsmith.page.tmpl", nil)
}

func handleCategoryPage(w http.ResponseWriter, r *http.Request, category string) {
	searchQuery := r.URL.Query().Get("q")
	var loadouts []Loadout

	if searchQuery != "" {
		rows, err := db.Query(`SELECT title, category, image_path FROM gunsmith_loadouts WHERE category = $1 AND title LIKE $2 ORDER BY id DESC`, category, "%"+searchQuery+"%")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var l Loadout
				if err := rows.Scan(&l.Title, &l.Category, &l.ImagePath); err == nil {
					loadouts = append(loadouts, l)
				}
			}
		}
	} else {
		rows, err := db.Query(`SELECT title, category, image_path FROM gunsmith_loadouts WHERE category = $1 ORDER BY id DESC`, category)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var l Loadout
				if err := rows.Scan(&l.Title, &l.Category, &l.ImagePath); err == nil {
					loadouts = append(loadouts, l)
				}
			}
		}
	}

	data := GunsmithPageData{
		CategoryName: category,
		SearchQuery:  searchQuery,
		Loadouts:     loadouts,
	}
	render(w, "gunsmith-category.page.tmpl", data)
}

func gunsmithAssault(w http.ResponseWriter, r *http.Request)  { handleCategoryPage(w, r, "Assault") }
func gunsmithSMG(w http.ResponseWriter, r *http.Request)      { handleCategoryPage(w, r, "SMG") }
func gunsmithSniper(w http.ResponseWriter, r *http.Request)   { handleCategoryPage(w, r, "Sniper") }
func gunsmithLMG(w http.ResponseWriter, r *http.Request)      { handleCategoryPage(w, r, "LMG") }
func gunsmithShotgun(w http.ResponseWriter, r *http.Request)  { handleCategoryPage(w, r, "Shotgun") }
func gunsmithMarksman(w http.ResponseWriter, r *http.Request) { handleCategoryPage(w, r, "Marksman") }

func gunsmithUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/gunsmith", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(10 << 20)
	cat := r.FormValue("category")
	title := r.FormValue("title")

	file, header, err := r.FormFile("loadout_image")
	if err != nil {
		log.Println("❌ [UPLOAD ERROR] File receive nahi hui:", err)
		http.Error(w, "Image is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	saveDir := filepath.Join("ui", "static", "uploads", "gunsmith")
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		log.Println("❌ [UPLOAD ERROR] Folder nahi ban paya:", err)
	}

	filename := strings.ReplaceAll(header.Filename, " ", "_")
	filePath := filepath.Join(saveDir, filename)
	dst, err := os.Create(filePath)
	if err != nil {
		log.Println("❌ [UPLOAD ERROR] File create nahi ho payi:", err)
		http.Redirect(w, r, "/gunsmith/"+strings.ToLower(cat), http.StatusSeeOther)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	dbPath := "/static/uploads/gunsmith/" + filename
	_, err = db.Exec("INSERT INTO gunsmith_loadouts (title, category, image_path) VALUES ($1, $2, $3)", title, cat, dbPath)
	if err != nil {
		log.Println("❌ [DB ERROR] Database mein save nahi hua:", err)
	} else {
		log.Println("✅ [SUCCESS] Nayi loadout save ho gayi! Category:", cat, "| Path:", dbPath)
	}

	redirectURL := "/gunsmith/" + strings.ToLower(cat)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
