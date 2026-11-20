package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestPadroesDeDesenvolvimento(t *testing.T) {
	c, err := Carregar(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Producao() || c.Endereco != ":8080" || c.CaminhoBanco != "data/aurea.db" || c.CookieSeguro {
		t.Errorf("padrões de dev inesperados: %+v", c)
	}
	if c.SessaoWeb != 8*time.Hour || c.SessaoApp != 30*24*time.Hour {
		t.Errorf("sessões: %v %v", c.SessaoWeb, c.SessaoApp)
	}
	if c.LoginTentativas != 5 || c.LoginJanela != 30*time.Minute || c.LoginBloqueio != 15*time.Minute {
		t.Errorf("limites de login: %d %v %v", c.LoginTentativas, c.LoginJanela, c.LoginBloqueio)
	}
	if c.BackupRetencao != 30 || c.BackupHorario != "03:00" || c.BackupRemoto != "" {
		t.Errorf("backup: %+v", c)
	}
}

func TestFusoEmbutidoSaoPaulo(t *testing.T) {
	c, err := Carregar(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	// 12:00 UTC em novembro = 09:00 em São Paulo (UTC-3, sem horário de verão desde 2019).
	h := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC).In(c.Local).Hour()
	if h != 9 {
		t.Fatalf("hora em São Paulo = %d, esperado 9", h)
	}
}

func TestProducaoExigeDominioECookieSeguroPorPadrao(t *testing.T) {
	if _, err := Carregar(env(map[string]string{"APP_ENV": "prod"})); err == nil || !strings.Contains(err.Error(), "DOMINIO") {
		t.Fatalf("esperava recusar prod sem DOMINIO, veio %v", err)
	}
	c, err := Carregar(env(map[string]string{"APP_ENV": "prod", "DOMINIO": "exemplo.test"}))
	if err != nil {
		t.Fatal(err)
	}
	if !c.CookieSeguro || !c.LogJSON {
		t.Errorf("prod deve ter cookie seguro e log JSON: %+v", c)
	}
	c, err = Carregar(env(map[string]string{"APP_ENV": "prod", "DOMINIO": "exemplo.test", "COOKIE_SECURE": "false"}))
	if err != nil || c.CookieSeguro {
		t.Fatalf("COOKIE_SECURE=false explícito deve valer: %v %+v", err, c)
	}
}

func TestErrosSaoReunidos(t *testing.T) {
	_, err := Carregar(env(map[string]string{
		"APP_ENV": "teste", "PORT": "abc", "LOGIN_TENTATIVAS": "0", "FUSO": "Marte/Olimpo",
		"BACKUP_HORARIO": "25:99", "COOKIE_SECURE": "talvez", "LOG_NIVEL": "gritando",
	}))
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, trecho := range []string{"APP_ENV", "PORT", "LOGIN_TENTATIVAS", "FUSO", "BACKUP_HORARIO", "COOKIE_SECURE", "LOG_NIVEL"} {
		if !strings.Contains(err.Error(), trecho) {
			t.Errorf("a mensagem não cita %s:\n%v", trecho, err)
		}
	}
}

func TestValoresPersonalizados(t *testing.T) {
	c, err := Carregar(env(map[string]string{
		"PORT": "9090", "BIND": "127.0.0.1", "DB_PATH": "/dados/a.db", "SESSION_TIMEOUT_HORAS": "2", "SESSION_APP_DIAS": "10",
		"LOGIN_TENTATIVAS": "3", "LOGIN_JANELA_MIN": "10", "LOGIN_BLOQUEIO_MIN": "20", "BACKUP_REMOTO": "gdrive:x", "BACKUP_RETENCAO_DIAS": "7", "BACKUP_HORARIO": "04:30",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Endereco != "127.0.0.1:9090" || c.CaminhoBanco != "/dados/a.db" || c.SessaoWeb != 2*time.Hour || c.SessaoApp != 240*time.Hour ||
		c.LoginTentativas != 3 || c.LoginJanela != 10*time.Minute || c.LoginBloqueio != 20*time.Minute ||
		c.BackupRemoto != "gdrive:x" || c.BackupRetencao != 7 || c.BackupHorario != "04:30" {
		t.Errorf("valores: %+v", c)
	}
}

func TestSenhaInicialDoAdmin(t *testing.T) {
	ruins := map[string]string{
		"curta":         "Ab1",
		"sem maiúscula": "senhasegura123",
		"sem minúscula": "SENHASEGURA123",
		"sem número":    "SenhaSegura!!",
		"conhecida":     "Admin@123",
		"longa demais":  "Aa1" + strings.Repeat("x", 80),
	}
	for nome, s := range ruins {
		if ValidarSenhaAdmin(s) == nil {
			t.Errorf("%s: %q deveria ser recusada", nome, s)
		}
		if _, err := Carregar(env(map[string]string{"ADMIN_SENHA_INICIAL": s})); err == nil {
			t.Errorf("%s: Carregar deveria recusar", nome)
		}
	}
	if err := ValidarSenhaAdmin("Bom-Segredo-2026"); err != nil {
		t.Errorf("senha boa recusada: %v", err)
	}
	// Não informar a senha é aceito na configuração: só é exigida quando o admin precisa ser criado (ver bootstrap).
	if _, err := Carregar(env(nil)); err != nil {
		t.Errorf("sem ADMIN_SENHA_INICIAL deve carregar: %v", err)
	}
}
