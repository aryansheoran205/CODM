package main

import (
	"context"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/sessions"
)

// --- SESSIONS SETUP ---
// Session ka use hum user ko login rakhne ke liye karte hain (jaise Google Login ke baad)
var store = sessions.NewCookieStore([]byte("codm-hub-super-secret-key"))

// --- DATA MODELS (Data ka Blueprint) ---
// Ye structs batate hain ki humara data kaisa dikhega.
type UserProfile struct {
	GameName string `json:"game_name"`
	UID      string `json:"uid"`
}

type ChatMessage struct {
	Username string
	Message  string
}

type GoogleUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Event struct {
	Title     string
	ImagePath string
}

// Gunsmith loadout ka blueprint
type Loadout struct {
	Title     string
	Category  string
	ImagePath string
}

// Gunsmith page par jo data bhejenge uska structure
type GunsmithPageData struct {
	CategoryName string
	SearchQuery  string
	Loadouts     []Loadout
}

// --- HELPER FUNCTIONS (Kaam aasan karne wale functions) ---

// Ye function kisi bhi data ko JSON file me save karta hai (database ki jagah file me)
func saveDataToFile(filename string, data interface{}) error {
	// os.OpenFile file ko kholta hai. Agar nahi hai toh bana deta hai (O_CREATE), aur naya data end me jodta hai (O_APPEND)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error opening file:", err)
		return err
	}
	// DEFER ka matlab hai: "Jab ye saveDataToFile function pura khatam ho jaye, tab is file ko aakhiri me Close kar dena."
	// Ye isliye lagate hain taaki file open na reh jaye aur memory bache.
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		log.Println("Error encoding JSON:", err)
		return err
	}
	return nil
}

// Ye function HTML files ko screen par dikhane (render) ka kaam karta hai
func render(w http.ResponseWriter, tmpl string, data interface{}) {
	// Base layout aur jo page dikhana hai, dono ko jodta hai
	files := []string{
		"./ui/html/base.layout.tmpl",
		"./ui/html/" + tmpl,
	}
	// HTML file ko padhta (parse) hai
	ts, err := template.ParseFiles(files...)
	if err != nil {
		log.Println("Parse error:", err.Error())
		http.Error(w, "Internal Server Error", 500)
		return
	}
	// Final HTML browser ko bhejta hai
	err = ts.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Println("Execute error:", err.Error())
		http.Error(w, "Internal Server Error", 500)
	}
}

// --- PAGE HANDLERS (Website ke alag-alag pages) ---

// Ye Home page (/) ka function hai
func home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	// Database se aakhiri 50 chat messages nikalta hai
	rows, err := db.Query(`SELECT username, message FROM chats WHERE room = 'home' ORDER BY id DESC LIMIT 50`)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	// Jab sab chats read ho jayein, tab database connection close kar dena (memory bachane ke liye)
	defer rows.Close()

	var chats []ChatMessage
	// Ek-ek karke rows padhta hai aur chats list me dalta hai
	for rows.Next() {
		var c ChatMessage
		if err := rows.Scan(&c.Username, &c.Message); err == nil {
			chats = append(chats, c)
		}
	}
	// Home page ko render karta hai aur chats ka data bhejta hai
	render(w, "home.page.tmpl", chats)
}

// Events page dikhane ke liye
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

// Chhote static pages
func reportBug(w http.ResponseWriter, r *http.Request)    { render(w, "report-bug.page.tmpl", nil) }
func reportPlayer(w http.ResponseWriter, r *http.Request) { render(w, "report-player.page.tmpl", nil) }
func suggestions(w http.ResponseWriter, r *http.Request)  { render(w, "suggestions.page.tmpl", nil) }

// User ki profile handle karne ke liye
func profile(w http.ResponseWriter, r *http.Request) {
	// Agar user form submit (POST) kar raha hai
	if r.Method == "POST" {
		r.ParseForm() // Form ka data read karo
		newUser := UserProfile{
			GameName: r.FormValue("game_name"),
			UID:      r.FormValue("uid"),
		}
		saveDataToFile("users_data.json", newUser)        // JSON me save kar do
		http.Redirect(w, r, "/profile", http.StatusFound) // Wapas profile page par bhej do
		return
	}
	// Agar form submit nahi ho raha, bas page dikhao
	render(w, "profile.page.tmpl", nil)
}

// --- CHAT & OAUTH (Google Login) ---
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

// Chat message database me save karne ke liye
func chatSend(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		message := r.FormValue("message")
		room := r.FormValue("room")

		// Session se check karta hai ki user logged in hai ya nahi
		session, _ := store.Get(r, "codm-session")
		username := "Guest" // Agar logged in nahi hai toh "Guest" naam de do
		if val, ok := session.Values["username"].(string); ok {
			username = val
		}

		// Message DB me daal do
		db.Exec(`INSERT INTO chats (room, username, message) VALUES (?, ?, ?)`, room, username, message)

		// Jis room se message aaya hai, wapas wahi bhej do
		if room == "home" {
			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			http.Redirect(w, r, "/team-up", http.StatusFound)
		}
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func googleLogin(w http.ResponseWriter, r *http.Request) {
	url := googleOauthConfig.AuthCodeURL("codm-random-state-string")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect) // User ko Google ke login page par bhejta hai
}

func googleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	if state != "codm-random-state-string" {
		http.Error(w, "State mismatch.", http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err == nil {
		client := googleOauthConfig.Client(context.Background(), token)
		response, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err == nil {
			defer response.Body.Close()
			userData, _ := io.ReadAll(response.Body)
			var gUser GoogleUser
			json.Unmarshal(userData, &gUser)

			// Login success hone ke baad session me naam save kar leta hai
			session, _ := store.Get(r, "codm-session")
			session.Values["username"] = gUser.Name
			session.Save(r, w)
		}
	}
	http.Redirect(w, r, "/team-up", http.StatusFound)
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
			db.Exec("INSERT INTO events (title, image_path) VALUES (?, ?)", title, dbPath)
		}
	}
	http.Redirect(w, r, "/events", http.StatusSeeOther)
}

// --- GUNSMITH HANDLERS ---

// Main Gunsmith menu page
func gunsmith(w http.ResponseWriter, r *http.Request) {
	render(w, "gunsmith.page.tmpl", nil)
}

// Ye function kisi bhi gun category (Assault, SMG) ka data DB se nikal kar lata hai
func handleCategoryPage(w http.ResponseWriter, r *http.Request, category string) {
	searchQuery := r.URL.Query().Get("q") // URL se dekhta hai ki user ne kuch search kiya hai kya? (?q=ak47)
	var loadouts []Loadout

	if searchQuery != "" {
		// Agar kuch search kiya hai, toh DB me LIKE laga kar dhundhta hai (Title match karta hai)
		rows, err := db.Query(`SELECT title, category, image_path FROM gunsmith_loadouts WHERE category = ? AND title LIKE ? ORDER BY id DESC`, category, "%"+searchQuery+"%")
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
		// Agar kuch search nahi kiya, toh uss category ki saari guns nikal lata hai
		rows, err := db.Query(`SELECT title, category, image_path FROM gunsmith_loadouts WHERE category = ? ORDER BY id DESC`, category)
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

	// Data HTML page ko bhejta hai
	data := GunsmithPageData{
		CategoryName: category,
		SearchQuery:  searchQuery,
		Loadouts:     loadouts,
	}
	render(w, "gunsmith-category.page.tmpl", data)
}

// Alag-alag category routes ke liye upar wala function reuse kiya gaya hai
func gunsmithAssault(w http.ResponseWriter, r *http.Request)  { handleCategoryPage(w, r, "Assault") }
func gunsmithSMG(w http.ResponseWriter, r *http.Request)      { handleCategoryPage(w, r, "SMG") }
func gunsmithSniper(w http.ResponseWriter, r *http.Request)   { handleCategoryPage(w, r, "Sniper") }
func gunsmithLMG(w http.ResponseWriter, r *http.Request)      { handleCategoryPage(w, r, "LMG") }
func gunsmithShotgun(w http.ResponseWriter, r *http.Request)  { handleCategoryPage(w, r, "Shotgun") }
func gunsmithMarksman(w http.ResponseWriter, r *http.Request) { handleCategoryPage(w, r, "Marksman") }

// 🔥 UPLOAD FUNCTION: Yahan se user ki image server par save hoti hai aur DB me entry hoti hai
func gunsmithUpload(w http.ResponseWriter, r *http.Request) {
	// Agar koi is URL par bina form submit kiye aata hai, toh usko wapas bhej do
	if r.Method != "POST" {
		http.Redirect(w, r, "/gunsmith", http.StatusSeeOther)
		return
	}

	// 10 << 20 ka matlab hai 10 MB. Ye allow karta hai max 10MB tak ki file aane dena.
	r.ParseMultipartForm(10 << 20)

	// Form se text data nikal rahe hain
	cat := r.FormValue("category")
	title := r.FormValue("title")

	// 1. FORM SE FILE UTHAO
	file, header, err := r.FormFile("loadout_image")
	if err != nil {
		log.Println("❌ [UPLOAD ERROR] File receive nahi hui:", err)
		http.Error(w, "Image is required", http.StatusBadRequest)
		return
	}
	// Defer yahan bhi lagaya taaki upload khatam hone ke baad temporary file upload close ho jaye
	defer file.Close()

	// 2. FOLDER BANAO AGAR NAHI HAI
	// filepath.Join folder ka path banata hai, jaise ui/static/uploads/gunsmith
	saveDir := filepath.Join("ui", "static", "uploads", "gunsmith")
	err = os.MkdirAll(saveDir, os.ModePerm) // os.ModePerm sabko read/write permission deta hai
	if err != nil {
		log.Println("❌ [UPLOAD ERROR] Folder nahi ban paya:", err)
	}

	// 3. FILE KE NAAM SE SPACES HATAO
	// Browser kabhi-kabhi spaces wale naam "my gun.png" ko "my%20gun.png" kar deta hai jisse error aati hai
	// Isliye hum space ko underscore "_" se badal dete hain.
	filename := strings.ReplaceAll(header.Filename, " ", "_")

	// 4. SERVER PAR NAYI KHALI FILE CREATE KARO
	filePath := filepath.Join(saveDir, filename)
	dst, err := os.Create(filePath)
	if err != nil {
		log.Println("❌ [UPLOAD ERROR] File create nahi ho payi:", err)
		http.Redirect(w, r, "/gunsmith/"+strings.ToLower(cat), http.StatusSeeOther)
		return
	}
	// Nayi file banne ke baad, function khatam hone par usko save/close karna zaroori hai
	defer dst.Close()

	// 5. DATA COPY KARO (Original file se nayi khali file me data daalo)
	io.Copy(dst, file)

	// 6. DATABASE MEIN ENTRY KARO
	// File server me save ho gayi, ab uska location DB me daal do taaki baad me HTML use dikha sake
	dbPath := "/static/uploads/gunsmith/" + filename
	_, err = db.Exec("INSERT INTO gunsmith_loadouts (title, category, image_path) VALUES (?, ?, ?)", title, cat, dbPath)
	if err != nil {
		log.Println("❌ [DB ERROR] Database mein save nahi hua:", err)
	} else {
		log.Println("✅ [SUCCESS] Nayi loadout save ho gayi! Category:", cat, "| Path:", dbPath)
	}

	// 7. WAPAS PAGE PAR REDIRECT KARO
	// Strings.ToLower("Assault") usko "assault" banayega, taaki URL match ho jaye (/gunsmith/assault)
	redirectURL := "/gunsmith/" + strings.ToLower(cat)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
