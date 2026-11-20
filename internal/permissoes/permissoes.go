// Package permissoes é o catálogo de permissões do sistema (21 módulos × 4 direitos), os perfis prontos e o mapa
// rota → módulo → ação, escritos a partir de Permissoes, PerfisAcesso e PermissaoInterceptor do sistema antigo (INVENTARIO §3).
package permissoes

// Acao é um dos 4 direitos de cada módulo.
type Acao string

const (
	Consulta    Acao = "CONSULTA"
	Inclusao    Acao = "INCLUSAO"
	Atualizacao Acao = "ATUALIZACAO"
	Exclusao    Acao = "EXCLUSAO"
)

// Acoes na ordem do enum do antigo.
var Acoes = []Acao{Consulta, Inclusao, Atualizacao, Exclusao}

// Modulos na ordem em que aparecem na tela de permissões.
var Modulos = []string{
	"PRODUTOS", "UNIDADES", "PESSOAS", "DOCUMENTOS", "VENDAS_TODOS", "COMISSOES", "AGENDA", "AGENDA_TODOS", "ESTOQUE", "RELATORIOS",
	"BI_EXECUTIVO", "BI_INSIGHTS", "METAS",
	"FIN_TITULOS", "FIN_CONTAS", "FIN_FLUXO", "FIN_DRE", "FIN_CADASTROS",
	"APP_VENDAS",
	"USUARIOS", "PERMISSOES",
}

// ModulosFinanceiro são os módulos do menu financeiro.
var ModulosFinanceiro = []string{"FIN_TITULOS", "FIN_CONTAS", "FIN_FLUXO", "FIN_DRE", "FIN_CADASTROS"}

// PadraoNovoUsuario: módulos em que um usuário novo já nasce com Consulta.
var PadraoNovoUsuario = []string{"PRODUTOS", "PESSOAS", "DOCUMENTOS", "ESTOQUE", "RELATORIOS"}

// LoginAdmin é o super-usuário: sempre tem tudo (comparado sem diferenciar maiúscula).
const LoginAdmin = "admin"

var rotulos = map[string]string{
	"PRODUTOS":      "Produtos",
	"UNIDADES":      "Unidades de medida",
	"PESSOAS":       "Pessoas",
	"DOCUMENTOS":    "Documentos",
	"VENDAS_TODOS":  "Ver vendas de todos os vendedores (só Consulta)",
	"COMISSOES":     "Salão — Comissões (só Consulta)",
	"AGENDA":        "Salão — Agenda (horários e bloqueios exigem Atualização)",
	"AGENDA_TODOS":  "Salão — Ver a agenda de todos os profissionais (só Consulta)",
	"ESTOQUE":       "Estoque",
	"RELATORIOS":    "Relatórios",
	"BI_EXECUTIVO":  "BI — Painel Executivo",
	"BI_INSIGHTS":   "BI — Insights",
	"METAS":         "Metas de vendas (Consulta: ver · Inclusão: definir)",
	"FIN_TITULOS":   "Financeiro — Títulos e baixas",
	"FIN_CONTAS":    "Financeiro — Contas e extrato",
	"FIN_FLUXO":     "Financeiro — Fluxo de caixa",
	"FIN_DRE":       "Financeiro — DRE",
	"FIN_CADASTROS": "Financeiro — Cadastros",
	"APP_VENDAS":    "Aplicativo de vendas",
	"USUARIOS":      "Usuários",
	"PERMISSOES":    "Permissões",
}

// Rotulo é o nome do módulo exibido nas telas e nas mensagens de permissão.
func Rotulo(modulo string) string {
	if r, ok := rotulos[modulo]; ok {
		return r
	}
	return modulo
}

// Autoridade é o texto MODULO_ACAO (ex.: PRODUTOS_INCLUSAO) usado na checagem.
func Autoridade(modulo string, acao Acao) string { return modulo + "_" + string(acao) }

// Conjunto guarda as autoridades de um usuário logado.
type Conjunto map[string]bool

// Pode diz se o conjunto tem a ação no módulo.
func (c Conjunto) Pode(modulo string, acao Acao) bool { return c[Autoridade(modulo, acao)] }

// Direito é uma linha de Direito_Usuario (um módulo de um usuário).
type Direito struct {
	Modulo                                    string
	Inclusao, Exclusao, Consulta, Atualizacao bool
}

// ParaUsuario monta as autoridades: o admin tem todas; os demais, só os direitos marcados.
func ParaUsuario(login string, direitos []Direito) Conjunto {
	c := Conjunto{}
	if equalFold(login, LoginAdmin) {
		for _, m := range Modulos {
			for _, a := range Acoes {
				c[Autoridade(m, a)] = true
			}
		}
		return c
	}
	for _, d := range direitos {
		if d.Consulta {
			c[Autoridade(d.Modulo, Consulta)] = true
		}
		if d.Inclusao {
			c[Autoridade(d.Modulo, Inclusao)] = true
		}
		if d.Atualizacao {
			c[Autoridade(d.Modulo, Atualizacao)] = true
		}
		if d.Exclusao {
			c[Autoridade(d.Modulo, Exclusao)] = true
		}
	}
	return c
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
