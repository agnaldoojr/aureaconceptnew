// Package config lê a configuração do sistema SOMENTE de variáveis de ambiente (INF-04) e recusa valores inseguros em produção (INF-05).
// Nenhuma senha, chave ou endereço fica no código; o modelo comentado é o arquivo .env.example na raiz do repositório.
package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // fuso America/Sao_Paulo embutido no binário (INF-07): a imagem final não precisa de tzdata
)

// Config é a configuração já validada.
type Config struct {
	Ambiente string // "dev" ou "prod" (variável APP_ENV)
	Porta    string // PORT: porta de escuta (padrão 8080)
	Endereco string // endereço completo de escuta, ex.: ":8080"

	CaminhoBanco string // DB_PATH
	Dominio      string // DOMINIO (usado pelo Caddy e em links absolutos; vazio em dev)

	CookieSeguro bool          // COOKIE_SECURE: cookie só por HTTPS (padrão true em prod; false em dev)
	SessaoWeb    time.Duration // SESSION_TIMEOUT: duração da sessão do navegador (padrão 8h)
	SessaoApp    time.Duration // SESSION_APP_DIAS: duração da sessão do aplicativo (padrão 30 dias)

	LoginTentativas int           // LOGIN_TENTATIVAS: falhas seguidas até bloquear (padrão 5)
	LoginJanela     time.Duration // LOGIN_JANELA_MIN: período em que as falhas são contadas (padrão 30 min)
	LoginBloqueio   time.Duration // LOGIN_BLOQUEIO_MIN: tempo de bloqueio (padrão 15 min)

	Fuso     string         // FUSO (padrão America/Sao_Paulo)
	Local    *time.Location // fuso carregado
	LogJSON  bool           // LOG_FORMATO=json (padrão json em prod, texto em dev)
	LogNivel string         // LOG_NIVEL: debug, info, warn, error

	AdminSenhaInicial string // ADMIN_SENHA_INICIAL: só usada se ainda não existir o usuário admin

	// Backup (usado pelos scripts e pelo subcomando de backup)
	BackupRemoto   string // BACKUP_REMOTO: remoto do rclone (ex.: gdrive:aurea-backups); vazio = backup desligado
	BackupRetencao int    // BACKUP_RETENCAO_DIAS (padrão 30)
	BackupHorario  string // BACKUP_HORARIO "HH:MM" (padrão 03:00)
}

// Producao diz se o perfil é de produção.
func (c *Config) Producao() bool { return c.Ambiente == "prod" }

// AplicarFuso faz o horário local do processo (usado em "hoje", "mês atual", DRE, metas e no SQLite) ser o do FUSO configurado.
func (c *Config) AplicarFuso() { time.Local = c.Local }

// Carregar monta a configuração a partir de getenv (normalmente os.Getenv). Reúne TODOS os problemas de uma vez,
// para quem configura o servidor corrigir tudo numa só tentativa.
func Carregar(getenv func(string) string) (*Config, error) {
	var erros []string
	falha := func(f string, a ...any) { erros = append(erros, fmt.Sprintf(f, a...)) }
	txt := func(nome, padrao string) string {
		if v := strings.TrimSpace(getenv(nome)); v != "" {
			return v
		}
		return padrao
	}
	inteiro := func(nome string, padrao, min, max int) int {
		v := strings.TrimSpace(getenv(nome))
		if v == "" {
			return padrao
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < min || n > max {
			falha("%s deve ser um número inteiro entre %d e %d (veio %q)", nome, min, max, v)
			return padrao
		}
		return n
	}
	booleano := func(nome string, padrao bool) bool {
		v := strings.ToLower(strings.TrimSpace(getenv(nome)))
		switch v {
		case "":
			return padrao
		case "1", "true", "sim", "yes", "on":
			return true
		case "0", "false", "nao", "não", "no", "off":
			return false
		}
		falha("%s deve ser true ou false (veio %q)", nome, v)
		return padrao
	}

	c := &Config{}

	c.Ambiente = strings.ToLower(txt("APP_ENV", "dev"))
	if c.Ambiente != "dev" && c.Ambiente != "prod" {
		falha("APP_ENV deve ser dev ou prod (veio %q)", c.Ambiente)
		c.Ambiente = "dev"
	}
	prod := c.Producao()

	c.Porta = txt("PORT", "8080")
	if n, err := strconv.Atoi(c.Porta); err != nil || n < 1 || n > 65535 {
		falha("PORT deve ser uma porta entre 1 e 65535 (veio %q)", c.Porta)
		c.Porta = "8080"
	}
	c.Endereco = net.JoinHostPort(txt("BIND", ""), c.Porta)

	c.CaminhoBanco = txt("DB_PATH", "data/aurea.db")
	c.Dominio = txt("DOMINIO", "")
	if prod && c.Dominio == "" {
		falha("DOMINIO é obrigatório em produção (APP_ENV=prod)")
	}

	// Em produção o cookie é seguro por padrão; para testar por http (sem domínio) é preciso escrever COOKIE_SECURE=false de propósito.
	c.CookieSeguro = booleano("COOKIE_SECURE", prod)
	horas := inteiro("SESSION_TIMEOUT_HORAS", 8, 1, 24*30)
	c.SessaoWeb = time.Duration(horas) * time.Hour
	dias := inteiro("SESSION_APP_DIAS", 30, 1, 365)
	c.SessaoApp = time.Duration(dias) * 24 * time.Hour

	c.LoginTentativas = inteiro("LOGIN_TENTATIVAS", 5, 1, 100)
	c.LoginJanela = time.Duration(inteiro("LOGIN_JANELA_MIN", 30, 1, 24*60)) * time.Minute
	c.LoginBloqueio = time.Duration(inteiro("LOGIN_BLOQUEIO_MIN", 15, 1, 24*60)) * time.Minute

	c.Fuso = txt("FUSO", "America/Sao_Paulo")
	loc, err := time.LoadLocation(c.Fuso)
	if err != nil {
		falha("FUSO inválido %q: %v", c.Fuso, err)
		loc = time.UTC
	}
	c.Local = loc

	c.LogJSON = strings.ToLower(txt("LOG_FORMATO", map[bool]string{true: "json", false: "texto"}[prod])) == "json"
	c.LogNivel = strings.ToLower(txt("LOG_NIVEL", "info"))
	switch c.LogNivel {
	case "debug", "info", "warn", "error":
	default:
		falha("LOG_NIVEL deve ser debug, info, warn ou error (veio %q)", c.LogNivel)
		c.LogNivel = "info"
	}

	c.AdminSenhaInicial = getenv("ADMIN_SENHA_INICIAL") // sem TrimSpace: espaços podem fazer parte da senha
	if c.AdminSenhaInicial != "" {
		if err := ValidarSenhaAdmin(c.AdminSenhaInicial); err != nil {
			falha("ADMIN_SENHA_INICIAL: %v", err)
		}
	}

	c.BackupRemoto = txt("BACKUP_REMOTO", "")
	c.BackupRetencao = inteiro("BACKUP_RETENCAO_DIAS", 30, 1, 3650)
	c.BackupHorario = txt("BACKUP_HORARIO", "03:00")
	if _, err := time.Parse("15:04", c.BackupHorario); err != nil {
		falha("BACKUP_HORARIO deve ser HH:MM (veio %q)", c.BackupHorario)
		c.BackupHorario = "03:00"
	}

	if len(erros) > 0 {
		return nil, errors.New("configuração inválida:\n  - " + strings.Join(erros, "\n  - "))
	}
	return c, nil
}

// senhasProibidas são senhas conhecidas que nunca podem ser a do admin (a primeira é a do schema.sql do sistema antigo).
var senhasProibidas = []string{"admin@123", "admin123", "administrador", "senha123", "mudar123"}

// ValidarSenhaAdmin aplica a regra mínima de complexidade da senha inicial do admin (D-01): 10+ caracteres,
// com maiúscula, minúscula e número, e fora da lista de senhas conhecidas.
func ValidarSenhaAdmin(senha string) error {
	if len([]rune(senha)) < 10 {
		return errors.New("precisa ter pelo menos 10 caracteres")
	}
	if len(senha) > 72 {
		return errors.New("precisa ter no máximo 72 bytes (limite do BCrypt)")
	}
	var maiuscula, minuscula, numero bool
	for _, r := range senha {
		switch {
		case r >= 'A' && r <= 'Z':
			maiuscula = true
		case r >= 'a' && r <= 'z':
			minuscula = true
		case r >= '0' && r <= '9':
			numero = true
		}
	}
	if !maiuscula || !minuscula || !numero {
		return errors.New("precisa ter letra maiúscula, letra minúscula e número")
	}
	baixa := strings.ToLower(senha)
	for _, p := range senhasProibidas {
		if baixa == p {
			return errors.New("é uma senha conhecida e não pode ser usada")
		}
	}
	return nil
}
