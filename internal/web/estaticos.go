package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
)

// arquivo é um estático embutido, já em memória (são ~1 MB no total; evita abrir o embed a cada pedido).
type arquivo struct {
	dados []byte
	hash  string // 32 caracteres hexadecimais do SHA-256 do conteúdo
	tipo  string
}

// Estaticos serve /css, /js, /img e /vendor com URL versionada pelo conteúdo (INF-08): /css/style-<hash>.css.
// O mesmo arquivo também responde no caminho sem o hash (o antigo Spring fazia igual).
type Estaticos struct {
	porCaminho map[string]*arquivo // "/css/style.css"
}

var tiposExtras = map[string]string{
	".webp": "image/webp", ".woff2": "font/woff2", ".woff": "font/woff", ".json": "application/json", ".svg": "image/svg+xml",
	".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".png": "image/png", ".ico": "image/x-icon", ".txt": "text/plain; charset=utf-8",
}

func tipoDe(nome string) string {
	ext := strings.ToLower(path.Ext(nome))
	if t, ok := tiposExtras[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

// NovosEstaticos lê tudo de raiz (a pasta "static" do embed) para a memória.
func NovosEstaticos(raiz fs.FS) (*Estaticos, error) {
	e := &Estaticos{porCaminho: map[string]*arquivo{}}
	err := fs.WalkDir(raiz, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(raiz, p)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		e.porCaminho["/"+p] = &arquivo{dados: b, hash: hex.EncodeToString(h[:16]), tipo: tipoDe(p)}
		return nil
	})
	return e, err
}

// URL devolve o endereço versionado de um estático ("/css/style.css" → "/css/style-<hash>.css").
// Arquivo inexistente é erro de programação (template com caminho errado): devolve o caminho cru e o teste de templates acusa.
func (e *Estaticos) URL(caminho string) string {
	a, ok := e.porCaminho[caminho]
	if !ok {
		return caminho
	}
	ext := path.Ext(caminho)
	return strings.TrimSuffix(caminho, ext) + "-" + a.hash + ext
}

// Existe diz se o caminho (sem hash) é um estático conhecido.
func (e *Estaticos) Existe(caminho string) bool { _, ok := e.porCaminho[caminho]; return ok }

var comHash = regexp.MustCompile(`^(.*)-([0-9a-f]{32})(\.[A-Za-z0-9]+)$`)

// Resolve acha o arquivo por "/css/style-<hash>.css" (versionado) ou "/css/style.css".
func (e *Estaticos) resolve(caminho string) (a *arquivo, versionado bool) {
	if a, ok := e.porCaminho[caminho]; ok {
		return a, false
	}
	if m := comHash.FindStringSubmatch(caminho); m != nil {
		if a, ok := e.porCaminho[m[1]+m[3]]; ok && a.hash == m[2] {
			return a, true
		}
	}
	return nil, false
}

// EEstatico diz se o caminho pertence às pastas públicas de estáticos.
func EEstatico(caminho string) bool {
	return strings.HasPrefix(caminho, "/css/") || strings.HasPrefix(caminho, "/js/") || strings.HasPrefix(caminho, "/img/") ||
		strings.HasPrefix(caminho, "/vendor/") || caminho == "/favicon.ico"
}

// ServeHTTP responde GET/HEAD de estáticos. Versionado: cache de 365 dias (o conteúdo nunca muda naquele endereço);
// vendor sem hash: 30 dias (versão fixa); o resto sem hash: sempre revalida por ETag.
func (e *Estaticos) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}
	caminho := r.URL.Path
	if caminho == "/favicon.ico" { // o antigo servia o logo; navegadores pedem /favicon.ico sem aviso
		caminho = "/img/logo-aurea.webp"
	}
	a, versionado := e.resolve(caminho)
	if a == nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", a.tipo)
	h.Set("ETag", `"`+a.hash+`"`)
	switch {
	case versionado:
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	case strings.HasPrefix(caminho, "/vendor/"):
		h.Set("Cache-Control", "public, max-age=2592000")
	default:
		h.Set("Cache-Control", "no-cache")
	}
	if r.Header.Get("If-None-Match") == `"`+a.hash+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Length", fmt.Sprint(len(a.dados)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(a.dados)
	}
}
