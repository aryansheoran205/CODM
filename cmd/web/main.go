package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	_ "github.com/lib/pq" // 🔥 PostgreSQL Driver
)

// --- GLOBAL VARIABLES (Dono files mein kaam aayenge) ---
var db *sql.DB
var store = sessions.NewCookieStore([]byte("codm-hub-super-secret-key"))

// --- INIT & SETUP ---
func initDB() {
	dsn := "postgres://codm_web:hathi@localhost:5432/codm_hub?sslmode=disable"
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Database open karne mein error: ", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Database Ping Error: ", err)
	}
	log.Println("[DB] PostgreSQL se connection ekdum SUCCESSFUL hai! 🎉")
}

// --- MAIN FUNCTION ---
func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// 🔥 Normal Pages
	mux.HandleFunc("/", home)
	mux.HandleFunc("/events", events)
	mux.HandleFunc("/team-up", teamUp)
	mux.HandleFunc("/report-bug", reportBug)
	mux.HandleFunc("/report-player", reportPlayer)
	mux.HandleFunc("/suggestions", suggestions)
	mux.HandleFunc("/profile", profile)

	// 🔥 Naye Normal Login/Register Pages (Google Hata Diya)
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/logout", logoutHandler)

	// 🔥 Gunsmith Routes
	mux.HandleFunc("/gunsmith", gunsmith)
	mux.HandleFunc("/gunsmith/assault", gunsmithAssault)
	mux.HandleFunc("/gunsmith/smg", gunsmithSMG)
	mux.HandleFunc("/gunsmith/sniper", gunsmithSniper)
	mux.HandleFunc("/gunsmith/lmg", gunsmithLMG)
	mux.HandleFunc("/gunsmith/shotgun", gunsmithShotgun)
	mux.HandleFunc("/gunsmith/marksman", gunsmithMarksman)
	mux.HandleFunc("/gunsmith/upload", gunsmithUpload)

	// 🔥 Action Routes
	mux.HandleFunc("/chat/send", chatSend)
	mux.HandleFunc("/add-event", handleAddEvent)

	log.Println("Starting CODM server on http://localhost:4000")
	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}
