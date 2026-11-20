package permissoes

import "strings"

// Decisao é o que o antigo PermissaoInterceptor exige para uma requisição.
type Decisao struct {
	// Verifica é falso quando a rota não passa pelo interceptor (login, dashboard, estáticos, /bi e visão geral do financeiro
	// — estes dois conferem no próprio handler — e /vendedor/api, que responde 403 JSON no handler).
	Verifica bool
	Modulo   string
	Acao     Acao
	// ExigeVendasTodos: BI e lucratividade somam as vendas de todos; sem VENDAS_TODOS (Consulta) o acesso é recusado
	// com a mensagem MensagemSomaVendas.
	ExigeVendasTodos bool
}

// MensagemSomaVendas é o aviso do antigo para quem tenta abrir BI/lucratividade só com as próprias vendas.
const MensagemSomaVendas = "Esta tela soma as vendas de todos os vendedores. Seu usuário só vê as próprias vendas."

// MensagemNegado monta "Você não tem permissão para {acessar|incluir registros em|alterar registros em|excluir registros de} {rótulo}."
func MensagemNegado(modulo string, acao Acao) string {
	var o string
	switch acao {
	case Consulta:
		o = "acessar"
	case Inclusao:
		o = "incluir registros em"
	case Atualizacao:
		o = "alterar registros em"
	case Exclusao:
		o = "excluir registros de"
	}
	return "Você não tem permissão para " + o + " " + Rotulo(modulo) + "."
}

// camposDeId: o formulário de cada módulo manda o código do registro num campo com este nome (vazio = registro novo).
var camposDeId = map[string]string{
	"PRODUTOS": "uprCodigo", "UNIDADES": "undCodigo", "PESSOAS": "upsCodigo", "DOCUMENTOS": "doctoCodigo",
	"AGENDA": "agdCodigo", "USUARIOS": "usuCodigo", "FIN_TITULOS": "titCodigo", "FIN_CONTAS": "cfnCodigo",
}

// splitJava reproduz String.split("/") do Java: remove as strings vazias do FINAL (o primeiro elemento vazio permanece).
func splitJava(caminho string) []string {
	seg := strings.Split(caminho, "/")
	for len(seg) > 0 && seg[len(seg)-1] == "" {
		seg = seg[:len(seg)-1]
	}
	return seg
}

// Decidir aplica a regra do antigo ao método e ao caminho (sem o contexto da aplicação). param devolve um campo do formulário/consulta.
func Decidir(metodo, caminho string, param func(string) string) Decisao {
	seg := splitJava(caminho)
	if len(seg) < 2 {
		return Decisao{}
	}
	if somaTodasAsVendas(seg) {
		// o antigo checa VENDAS_TODOS primeiro e, passando, ainda aplica o módulo da rota (RELATORIOS), se houver
		d := decidirModulo(metodo, seg, param)
		d.ExigeVendasTodos = true
		return d
	}
	return decidirModulo(metodo, seg, param)
}

func decidirModulo(metodo string, seg []string, param func(string) string) Decisao {
	modulo := moduloDe(seg)
	if modulo == "" {
		return Decisao{}
	}
	return Decisao{Verifica: true, Modulo: modulo, Acao: acaoDe(modulo, metodo, seg, param)}
}

func somaTodasAsVendas(seg []string) bool {
	return seg[1] == "bi" || (seg[1] == "relatorios" && len(seg) > 2 && seg[2] == "lucratividade")
}

func moduloDe(seg []string) string {
	switch seg[1] {
	case "produtos":
		return "PRODUTOS"
	case "unidades":
		return "UNIDADES"
	case "pessoas":
		return "PESSOAS"
	case "documentos":
		return "DOCUMENTOS"
	case "estoque":
		return "ESTOQUE"
	case "comissoes":
		return "COMISSOES"
	case "metas":
		return "METAS"
	case "agenda":
		return "AGENDA"
	case "vendedor":
		if len(seg) > 2 && seg[2] == "api" {
			return ""
		}
		return "APP_VENDAS"
	case "relatorios":
		return "RELATORIOS"
	case "usuarios":
		if len(seg) > 2 && seg[2] == "permissoes" {
			return "PERMISSOES"
		}
		return "USUARIOS"
	case "financeiro":
		if len(seg) < 3 {
			return ""
		}
		switch seg[2] {
		case "titulos":
			return "FIN_TITULOS"
		case "contas":
			return "FIN_CONTAS"
		case "fluxo":
			return "FIN_FLUXO"
		case "dre":
			return "FIN_DRE"
		case "formas", "tipos", "categorias":
			return "FIN_CADASTROS"
		}
	}
	return ""
}

func acaoDe(modulo, metodo string, seg []string, param func(string) string) Acao {
	post := strings.EqualFold(metodo, "POST")
	if modulo == "PERMISSOES" {
		if post {
			return Atualizacao
		}
		return Consulta
	}
	// no financeiro a ação vem depois do submódulo: /financeiro/titulos/baixar/5
	fin := seg[1] == "financeiro"
	idx := 2
	if fin {
		idx = 3
	}
	sub := ""
	if len(seg) > idx {
		sub = seg[idx]
	}
	// horários e bloqueios da agenda: ver = Consulta; gravar = Atualização; apagar = Exclusão
	if modulo == "AGENDA" && len(seg) > 2 && (seg[2] == "bloqueios" || seg[2] == "horarios") {
		a := ""
		if len(seg) > 3 {
			a = seg[3]
		}
		switch a {
		case "salvar":
			return Atualizacao
		case "excluir":
			return Exclusao
		}
		return Consulta
	}
	switch sub {
	case "novo", "nova", "transferir":
		return Inclusao
	case "editar", "status", "baixar", "baixar-lote", "conciliar":
		return Atualizacao
	case "excluir", "cancelar", "estornar", "estornar-transferencia":
		return Exclusao
	case "salvar":
		if temId(modulo, seg, param) {
			return Atualizacao
		}
		return Inclusao
	}
	return Consulta
}

func temId(modulo string, seg []string, param func(string) string) bool {
	campo := camposDeId[modulo]
	if modulo == "FIN_CADASTROS" {
		switch seg[2] {
		case "formas":
			campo = "fpgCodigo"
		case "tipos":
			campo = "tptCodigo"
		default:
			campo = "catCodigo"
		}
	}
	if campo == "" || param == nil {
		return false
	}
	return strings.TrimSpace(param(campo)) != ""
}
