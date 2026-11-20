package web

import (
	"strings"
	"sync"
	"time"
)

// LimiteLogin freia tentativas de adivinhar senha (tela de login e, mais adiante, a API do app). Port do LoginThrottle do sistema antigo.
//
// Dois contadores, ambos por IP: um por IP + login (poucas tentativas: protege cada conta) e um geral do IP (mais folgado: pega quem
// testa muitos logins). Ao estourar, o IP fica bloqueado pelo tempo configurado. Login correto zera só o contador daquele IP + login.
// Uma falha antiga (mais velha que o tempo de bloqueio) recomeça a contagem. Fica na memória: reiniciar o sistema libera todos.
type LimiteLogin struct {
	maxPorLogin, maxPorIP int
	bloqueio              time.Duration
	agora                 func() time.Time

	mu        sync.Mutex
	registros map[string]registro
}

type registro struct {
	falhas       int
	ultimaFalha  time.Time
	bloqueadoAte time.Time
}

// NovoLimiteLogin cria o limitador. agora pode ser nil (usa time.Now).
func NovoLimiteLogin(maxPorLogin, maxPorIP int, bloqueio time.Duration, agora func() time.Time) *LimiteLogin {
	if agora == nil {
		agora = time.Now
	}
	return &LimiteLogin{maxPorLogin: maxPorLogin, maxPorIP: maxPorIP, bloqueio: bloqueio, agora: agora, registros: map[string]registro{}}
}

// Bloqueado diz se este IP (ou este IP com este login) está bloqueado agora.
func (l *LimiteLogin) Bloqueado(ip, login string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	agora := l.agora()
	return l.ativo(l.registros[chaveIP(ip)], agora) || l.ativo(l.registros[chaveLogin(ip, login)], agora)
}

// Falha registra uma tentativa errada.
func (l *LimiteLogin) Falha(ip, login string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.registrar(chaveIP(ip), l.maxPorIP)
	l.registrar(chaveLogin(ip, login), l.maxPorLogin)
	if len(l.registros) > 10_000 {
		agora := l.agora()
		for k, r := range l.registros {
			if agora.Sub(r.ultimaFalha) > l.bloqueio {
				delete(l.registros, k)
			}
		}
	}
}

// Sucesso zera o contador do par IP + login.
func (l *LimiteLogin) Sucesso(ip, login string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.registros, chaveLogin(ip, login))
}

func (l *LimiteLogin) registrar(chave string, limite int) {
	agora := l.agora()
	r := l.registros[chave]
	falhas := r.falhas + 1
	if r.falhas == 0 || agora.Sub(r.ultimaFalha) > l.bloqueio {
		falhas = 1
	}
	var ate time.Time
	if falhas >= limite {
		ate = agora.Add(l.bloqueio)
	}
	l.registros[chave] = registro{falhas: falhas, ultimaFalha: agora, bloqueadoAte: ate}
}

func (l *LimiteLogin) ativo(r registro, agora time.Time) bool { return r.bloqueadoAte.After(agora) }

func chaveIP(ip string) string { return "ip:" + ip }

// chaveLogin usa minúsculas sem espaços nas pontas, como o antigo (Locale.ROOT).
func chaveLogin(ip, login string) string {
	return "u:" + ip + "|" + strings.ToLower(strings.TrimSpace(login))
}
