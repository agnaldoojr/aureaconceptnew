package permissoes

// Perfil é um atalho que marca de uma vez as permissões típicas de uma função (módulo ausente = sem acesso).
type Perfil struct {
	Codigo, Nome, Descricao string
	Direitos                map[string][]Acao
}

// AcoesDe devolve as ações liberadas no módulo.
func (p Perfil) AcoesDe(modulo string) []Acao { return p.Direitos[modulo] }

// Tem diz se o perfil libera a ação no módulo.
func (p Perfil) Tem(modulo string, acao Acao) bool {
	for _, a := range p.Direitos[modulo] {
		if a == acao {
			return true
		}
	}
	return false
}

// Códigos dos perfis prontos.
const (
	VendedorLoja      = "VENDEDOR_LOJA"
	ProfissionalSalao = "PROFISSIONAL_SALAO"
	Recepcao          = "RECEPCAO"
	Gerencia          = "GERENCIA"
)

// Perfis são os 4 perfis prontos, na ordem do antigo.
var Perfis = []Perfil{
	{
		Codigo: VendedorLoja, Nome: "Vendedor da loja",
		Descricao: "Lança e acompanha só as próprias vendas; consulta produtos e estoque; cadastra clientes. Usa o aplicativo.",
		Direitos: map[string][]Acao{
			"PRODUTOS":   {Consulta},
			"UNIDADES":   {Consulta},
			"ESTOQUE":    {Consulta},
			"PESSOAS":    {Consulta, Inclusao, Atualizacao},
			"DOCUMENTOS": {Consulta, Inclusao, Atualizacao},
			"APP_VENDAS": {Consulta, Inclusao},
		},
	},
	{
		Codigo: ProfissionalSalao, Nome: "Profissional do salão",
		Descricao: "Lança e acompanha só os próprios atendimentos, a própria agenda e a própria comissão; consulta serviços e produtos; cadastra clientes.",
		Direitos: map[string][]Acao{
			"PRODUTOS":   {Consulta},
			"UNIDADES":   {Consulta},
			"ESTOQUE":    {Consulta},
			"PESSOAS":    {Consulta, Inclusao, Atualizacao},
			"DOCUMENTOS": {Consulta, Inclusao, Atualizacao},
			"COMISSOES":  {Consulta},
			"AGENDA":     {Consulta, Inclusao, Atualizacao}, // só a própria agenda
		},
	},
	{
		Codigo: Recepcao, Nome: "Recepção",
		Descricao: "Cuida da agenda de todos os profissionais (marcar, mudar, cancelar, horários e bloqueios) e cadastra clientes. Não vê vendas.",
		Direitos: map[string][]Acao{
			"AGENDA":       {Consulta, Inclusao, Atualizacao, Exclusao},
			"AGENDA_TODOS": {Consulta},
			"PESSOAS":      {Consulta, Inclusao, Atualizacao},
			"PRODUTOS":     {Consulta},
		},
	},
	gerencia(),
}

// gerencia: tudo, inclusive ver as vendas de todos, menos a tela de permissões (que fica com o administrador).
func gerencia() Perfil {
	d := map[string][]Acao{}
	for _, m := range Modulos {
		switch {
		case m == "PERMISSOES":
			continue
		case m == "VENDAS_TODOS" || m == "COMISSOES" || m == "AGENDA_TODOS":
			d[m] = []Acao{Consulta}
		case m == "METAS":
			d[m] = []Acao{Consulta, Inclusao} // ver e definir metas
		default:
			d[m] = []Acao{Consulta, Inclusao, Atualizacao, Exclusao}
		}
	}
	return Perfil{
		Codigo: Gerencia, Nome: "Gerência",
		Descricao: "Acesso a tudo, inclusive vendas de todos os vendedores, BI, financeiro e usuários (menos Permissões).",
		Direitos:  d,
	}
}

// BuscarPerfil acha um perfil pronto pelo código.
func BuscarPerfil(codigo string) (Perfil, bool) {
	for _, p := range Perfis {
		if p.Codigo == codigo {
			return p, true
		}
	}
	return Perfil{}, false
}
