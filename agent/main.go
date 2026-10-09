// weebio-agent serves the Weebio web UI and the box's local API on 127.0.0.1:
//
//	/                     the custom stremio-web build
//	/weebio/boot          splash: waits for internet, then opens Stremio or Wi-Fi setup
//	/weebio/setup         Wi-Fi setup page
//	/weebio/themes/*.css  colour themes
//	/weebio/api/...       net status, Wi-Fi, rendered addon list, version
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web
var embedded embed.FS

type server struct {
	wifi        *Wifi
	online      *Online
	addonsPath  string
	secretsPath string
	version     string
}

func main() {
	root := bundleRoot()
	listen := flag.String("listen", "127.0.0.1:8090", "address to serve on (keep it on loopback)")
	webDir := flag.String("web", filepath.Join(root, "web"), "stremio-web build directory")
	themesDir := flag.String("themes", filepath.Join(root, "themes"), "theme CSS directory")
	addons := flag.String("addons", filepath.Join(root, "addons.json"), "addon list with {{KEY}} placeholders")
	secrets := flag.String("secrets", "/etc/weebio/secrets.env", "KEY=value secrets file")
	probe := flag.String("probe", "https://v3-cinemeta.strem.io/manifest.json", "URL that must be reachable to count as online")
	flag.Parse()

	version, _ := os.ReadFile(filepath.Join(root, "VERSION"))
	s := &server{
		wifi:        &Wifi{run: execRunner},
		online:      NewOnline(*probe),
		addonsPath:  *addons,
		secretsPath: *secrets,
		version:     strings.TrimSpace(string(version)),
	}

	log.Printf("weebio-agent %s listening on %s (web=%s)", s.version, *listen, *webDir)
	srv := &http.Server{Addr: *listen, Handler: s.routes(*webDir, *themesDir), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// bundleRoot is the release directory this binary lives in (<root>/bin/weebio-agent).
func bundleRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(filepath.Dir(exe))
}

func (s *server) routes(webDir, themesDir string) http.Handler {
	pages, _ := fs.Sub(embedded, "web")
	mux := http.NewServeMux()

	mux.Handle("GET /weebio/", noCache(http.StripPrefix("/weebio/", pageServer(pages))))
	mux.Handle("GET /weebio/themes/", http.StripPrefix("/weebio/themes/", http.FileServer(http.Dir(themesDir))))

	mux.HandleFunc("GET /weebio/api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
	})
	mux.HandleFunc("GET /weebio/api/net", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"online": s.online.Check(r.Context())})
	})
	mux.HandleFunc("GET /weebio/api/wifi/networks", func(w http.ResponseWriter, r *http.Request) {
		networks, err := s.wifi.Networks(r.Context(), r.URL.Query().Get("rescan") == "1")
		if err != nil {
			writeError(w, http.StatusBadGateway, "scan_failed", err)
			return
		}
		writeJSON(w, http.StatusOK, networks)
	})
	mux.HandleFunc("POST /weebio/api/wifi/connect", s.connect)
	mux.HandleFunc("GET /weebio/api/addons", func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile(s.addonsPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "addons_unreadable", err)
			return
		}
		secrets, err := loadSecrets(s.secretsPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "secrets_unreadable", err)
			return
		}
		list, err := renderAddons(raw, secrets)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "addons_invalid", err)
			return
		}
		w.Header().Set("Cache-Control", "no-store") // contains secrets
		writeJSON(w, http.StatusOK, list)
	})

	// stremio-web registers a service worker that would keep serving a cached UI after an update;
	// refusing the script keeps every launch on the installed version.
	mux.HandleFunc("GET /service-worker.js", http.NotFound)
	mux.Handle("GET /", webServer(webDir))
	return mux
}

func (s *server) connect(w http.ResponseWriter, r *http.Request) {
	// Only our own pages send this header; a cross-origin page can't without a CORS preflight we never grant.
	if r.Header.Get("X-Weebio") != "1" {
		writeError(w, http.StatusForbidden, "forbidden", errors.New("missing X-Weebio header"))
		return
	}
	var req struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.SSID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", errors.New("ssid required"))
		return
	}
	if err := s.wifi.Connect(r.Context(), req.SSID, req.Password); err != nil {
		code := "connect_failed"
		if errors.Is(err, ErrWrongPassword) {
			code = "wrong_password"
		}
		log.Printf("wifi connect %q failed: %v", req.SSID, err)
		writeError(w, http.StatusBadGateway, code, err)
		return
	}
	s.online.Forget()
	log.Printf("wifi connected to %q", req.SSID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// pageServer serves boot.html for /weebio/boot etc.
func pageServer(pages fs.FS) http.Handler {
	files := http.FileServer(http.FS(pages))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" {
			http.Redirect(w, r, "/weebio/boot", http.StatusFound)
			return
		}
		if !strings.Contains(r.URL.Path, ".") {
			r.URL.Path += ".html"
		}
		files.ServeHTTP(w, r)
	})
}

// webServer serves the stremio-web build: index.html is never cached, hashed assets are.
func webServer(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}
