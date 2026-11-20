package permissoes

import "testing"

func campos(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// Cada linha é uma rota real do sistema antigo (INVENTARIO §1) com o módulo e a ação que o PermissaoInterceptor exigia.
func TestMapaDeRotasIgualAoInterceptorAntigo(t *testing.T) {
	novo := map[string]string{}
	edicao := func(campo string) map[string]string { return map[string]string{campo: "7"} }
	casos := []struct {
		metodo, caminho string
		form            map[string]string
		verifica        bool
		modulo          string
		acao            Acao
	}{
		// rotas sem checagem no interceptor
		{"GET", "/login", nil, false, "", ""},
		{"GET", "/dashboard", nil, false, "", ""},
		{"GET", "/", nil, false, "", ""},
		{"GET", "/css/style-1.css", nil, false, "", ""},
		{"GET", "/financeiro", nil, false, "", ""},
		{"GET", "/vendedor/api/me", nil, false, "", ""},
		{"POST", "/vendedor/api/pedidos", nil, false, "", ""},
		// produtos
		{"GET", "/produtos", nil, true, "PRODUTOS", Consulta},
		{"GET", "/produtos/novo", nil, true, "PRODUTOS", Inclusao},
		{"GET", "/produtos/editar/3", nil, true, "PRODUTOS", Atualizacao},
		{"POST", "/produtos/salvar", novo, true, "PRODUTOS", Inclusao},
		{"POST", "/produtos/salvar", edicao("uprCodigo"), true, "PRODUTOS", Atualizacao},
		{"POST", "/produtos/salvar", map[string]string{"uprCodigo": "  "}, true, "PRODUTOS", Inclusao},
		{"POST", "/produtos/status/3", nil, true, "PRODUTOS", Atualizacao},
		{"POST", "/produtos/excluir/3", nil, true, "PRODUTOS", Exclusao},
		// cadastros
		{"GET", "/unidades/novo", nil, true, "UNIDADES", Inclusao},
		{"POST", "/unidades/salvar", edicao("undCodigo"), true, "UNIDADES", Atualizacao},
		{"POST", "/pessoas/salvar", novo, true, "PESSOAS", Inclusao},
		{"GET", "/pessoas/nova", nil, true, "PESSOAS", Inclusao},
		{"POST", "/pessoas/excluir/2", nil, true, "PESSOAS", Exclusao},
		{"POST", "/usuarios/salvar", edicao("usuCodigo"), true, "USUARIOS", Atualizacao},
		{"POST", "/usuarios/status/2", nil, true, "USUARIOS", Atualizacao},
		// permissões: ver = Consulta, qualquer POST = Atualização
		{"GET", "/usuarios/permissoes/2", nil, true, "PERMISSOES", Consulta},
		{"POST", "/usuarios/permissoes/2", nil, true, "PERMISSOES", Atualizacao},
		{"POST", "/usuarios/permissoes/2/perfil", nil, true, "PERMISSOES", Atualizacao},
		// documentos e estoque
		{"GET", "/documentos", nil, true, "DOCUMENTOS", Consulta},
		{"GET", "/documentos/novo", nil, true, "DOCUMENTOS", Inclusao},
		{"GET", "/documentos/visualizar/9", nil, true, "DOCUMENTOS", Consulta},
		{"GET", "/documentos/editar/9", nil, true, "DOCUMENTOS", Atualizacao},
		{"POST", "/documentos/salvar", edicao("doctoCodigo"), true, "DOCUMENTOS", Atualizacao},
		{"POST", "/documentos/salvar", novo, true, "DOCUMENTOS", Inclusao},
		{"POST", "/documentos/excluir/9", nil, true, "DOCUMENTOS", Exclusao},
		{"GET", "/estoque", nil, true, "ESTOQUE", Consulta},
		{"GET", "/relatorios", nil, true, "RELATORIOS", Consulta},
		{"GET", "/relatorios/estoque", nil, true, "RELATORIOS", Consulta},
		{"GET", "/relatorios/documentos", nil, true, "RELATORIOS", Consulta},
		{"GET", "/comissoes", nil, true, "COMISSOES", Consulta},
		{"GET", "/metas", nil, true, "METAS", Consulta},
		{"POST", "/metas/salvar", nil, true, "METAS", Inclusao}, // "Inclusão: definir"
		// agenda
		{"GET", "/agenda", nil, true, "AGENDA", Consulta},
		{"GET", "/agenda/novo", nil, true, "AGENDA", Inclusao},
		{"GET", "/agenda/editar/4", nil, true, "AGENDA", Atualizacao},
		{"GET", "/agenda/faturar/4", nil, true, "AGENDA", Consulta},
		{"POST", "/agenda/salvar", novo, true, "AGENDA", Inclusao},
		{"POST", "/agenda/salvar", edicao("agdCodigo"), true, "AGENDA", Atualizacao},
		{"POST", "/agenda/status/4", nil, true, "AGENDA", Atualizacao},
		{"POST", "/agenda/excluir/4", nil, true, "AGENDA", Exclusao},
		{"GET", "/agenda/bloqueios", nil, true, "AGENDA", Consulta},
		{"POST", "/agenda/bloqueios/salvar", nil, true, "AGENDA", Atualizacao},
		{"POST", "/agenda/bloqueios/excluir/1", nil, true, "AGENDA", Exclusao},
		{"GET", "/agenda/horarios", nil, true, "AGENDA", Consulta},
		{"POST", "/agenda/horarios/salvar", nil, true, "AGENDA", Atualizacao},
		// financeiro
		{"GET", "/financeiro/titulos", nil, true, "FIN_TITULOS", Consulta},
		{"GET", "/financeiro/titulos/novo", nil, true, "FIN_TITULOS", Inclusao},
		{"GET", "/financeiro/titulos/editar/5", nil, true, "FIN_TITULOS", Atualizacao},
		{"POST", "/financeiro/titulos/salvar", novo, true, "FIN_TITULOS", Inclusao},
		{"POST", "/financeiro/titulos/salvar", edicao("titCodigo"), true, "FIN_TITULOS", Atualizacao},
		{"GET", "/financeiro/titulos/visualizar/5", nil, true, "FIN_TITULOS", Consulta},
		{"POST", "/financeiro/titulos/baixar/5", nil, true, "FIN_TITULOS", Atualizacao},
		{"POST", "/financeiro/titulos/baixar-lote", nil, true, "FIN_TITULOS", Atualizacao},
		{"POST", "/financeiro/titulos/estornar/8", nil, true, "FIN_TITULOS", Exclusao},
		{"POST", "/financeiro/titulos/cancelar/5", nil, true, "FIN_TITULOS", Exclusao},
		{"GET", "/financeiro/contas", nil, true, "FIN_CONTAS", Consulta},
		{"GET", "/financeiro/contas/extrato/1", nil, true, "FIN_CONTAS", Consulta},
		{"POST", "/financeiro/contas/conciliar/1", nil, true, "FIN_CONTAS", Atualizacao},
		{"GET", "/financeiro/contas/transferir", nil, true, "FIN_CONTAS", Inclusao},
		{"POST", "/financeiro/contas/transferir", nil, true, "FIN_CONTAS", Inclusao},
		{"POST", "/financeiro/contas/estornar-transferencia/2", nil, true, "FIN_CONTAS", Exclusao},
		{"POST", "/financeiro/contas/salvar", edicao("cfnCodigo"), true, "FIN_CONTAS", Atualizacao},
		{"GET", "/financeiro/fluxo", nil, true, "FIN_FLUXO", Consulta},
		{"GET", "/financeiro/dre", nil, true, "FIN_DRE", Consulta},
		{"GET", "/financeiro/formas", nil, true, "FIN_CADASTROS", Consulta},
		{"POST", "/financeiro/formas/salvar", edicao("fpgCodigo"), true, "FIN_CADASTROS", Atualizacao},
		{"POST", "/financeiro/formas/salvar", edicao("catCodigo"), true, "FIN_CADASTROS", Inclusao}, // campo de outro cadastro não vale
		{"POST", "/financeiro/tipos/salvar", edicao("tptCodigo"), true, "FIN_CADASTROS", Atualizacao},
		{"POST", "/financeiro/categorias/salvar", edicao("catCodigo"), true, "FIN_CADASTROS", Atualizacao},
		{"POST", "/financeiro/categorias/excluir/1", nil, true, "FIN_CADASTROS", Exclusao},
		{"GET", "/financeiro/relatorio-inexistente", nil, false, "", ""},
		// modo vendedor
		{"GET", "/vendedor", nil, true, "APP_VENDAS", Consulta},
		// barra final e caminhos curtos seguem o String.split do Java
		{"GET", "/produtos/", nil, true, "PRODUTOS", Consulta},
		{"GET", "", nil, false, "", ""},
	}
	for _, c := range casos {
		d := Decidir(c.metodo, c.caminho, campos(c.form))
		if d.Verifica != c.verifica || d.Modulo != c.modulo || d.Acao != c.acao {
			t.Errorf("%s %s %v: obteve {verifica=%v %s %s}, esperado {verifica=%v %s %s}", c.metodo, c.caminho, c.form, d.Verifica, d.Modulo, d.Acao, c.verifica, c.modulo, c.acao)
		}
	}
}

func TestBIeLucratividadeExigemVendasTodos(t *testing.T) {
	for _, caminho := range []string{"/bi", "/bi/", "/relatorios/lucratividade"} {
		if d := Decidir("GET", caminho, nil); !d.ExigeVendasTodos {
			t.Errorf("%s deveria exigir VENDAS_TODOS", caminho)
		}
	}
	// a lucratividade ainda passa pelo módulo RELATORIOS; o BI é conferido no handler
	if d := Decidir("GET", "/relatorios/lucratividade", nil); !d.Verifica || d.Modulo != "RELATORIOS" || d.Acao != Consulta {
		t.Errorf("lucratividade: %+v", d)
	}
	if d := Decidir("GET", "/bi", nil); d.Verifica {
		t.Errorf("/bi não passa pelo módulo do interceptor: %+v", d)
	}
	for _, caminho := range []string{"/relatorios/estoque", "/documentos", "/financeiro/dre"} {
		if Decidir("GET", caminho, nil).ExigeVendasTodos {
			t.Errorf("%s não deveria exigir VENDAS_TODOS", caminho)
		}
	}
}

func TestMensagensDeNegacao(t *testing.T) {
	casos := []struct {
		modulo string
		acao   Acao
		quer   string
	}{
		{"DOCUMENTOS", Consulta, "Você não tem permissão para acessar Documentos."},
		{"PRODUTOS", Inclusao, "Você não tem permissão para incluir registros em Produtos."},
		{"PESSOAS", Atualizacao, "Você não tem permissão para alterar registros em Pessoas."},
		{"FIN_TITULOS", Exclusao, "Você não tem permissão para excluir registros de Financeiro — Títulos e baixas."},
		{"PERMISSOES", Consulta, "Você não tem permissão para acessar Permissões."},
	}
	for _, c := range casos {
		if got := MensagemNegado(c.modulo, c.acao); got != c.quer {
			t.Errorf("%s/%s: %q, esperado %q", c.modulo, c.acao, got, c.quer)
		}
	}
	if MensagemSomaVendas != "Esta tela soma as vendas de todos os vendedores. Seu usuário só vê as próprias vendas." {
		t.Error("mensagem de soma das vendas mudou")
	}
}

func TestCatalogoTem21ModulosE4Acoes(t *testing.T) {
	if len(Modulos) != 21 {
		t.Errorf("%d módulos, esperado 21", len(Modulos))
	}
	if len(Acoes) != 4 {
		t.Errorf("%d ações, esperado 4", len(Acoes))
	}
	vistos := map[string]bool{}
	for _, m := range Modulos {
		if vistos[m] {
			t.Errorf("módulo repetido: %s", m)
		}
		vistos[m] = true
		if Rotulo(m) == m {
			t.Errorf("módulo %s sem rótulo", m)
		}
	}
}

func TestAdminSempreTemTudo(t *testing.T) {
	for _, login := range []string{"admin", "ADMIN", "Admin"} {
		c := ParaUsuario(login, nil)
		for _, m := range Modulos {
			for _, a := range Acoes {
				if !c.Pode(m, a) {
					t.Fatalf("%s sem %s_%s", login, m, a)
				}
			}
		}
	}
	if ParaUsuario("administrador", nil).Pode("PRODUTOS", Consulta) {
		t.Error("só o login 'admin' é super-usuário")
	}
}

func TestPermissoesSoVemDosDireitosMarcados(t *testing.T) {
	c := ParaUsuario("maria", []Direito{{Modulo: "PRODUTOS", Consulta: true, Atualizacao: true}, {Modulo: "PESSOAS", Inclusao: true}})
	if !c.Pode("PRODUTOS", Consulta) || !c.Pode("PRODUTOS", Atualizacao) || c.Pode("PRODUTOS", Inclusao) || c.Pode("PRODUTOS", Exclusao) {
		t.Error("PRODUTOS errado")
	}
	if !c.Pode("PESSOAS", Inclusao) || c.Pode("PESSOAS", Consulta) || c.Pode("DOCUMENTOS", Consulta) {
		t.Error("PESSOAS/DOCUMENTOS errado")
	}
}

func TestPerfisProntos(t *testing.T) {
	if len(Perfis) != 4 {
		t.Fatalf("%d perfis, esperado 4", len(Perfis))
	}
	g, _ := BuscarPerfil(Gerencia)
	if g.Tem("PERMISSOES", Consulta) {
		t.Error("a gerência não tem a tela de permissões")
	}
	for _, m := range []string{"VENDAS_TODOS", "COMISSOES", "AGENDA_TODOS"} {
		if len(g.AcoesDe(m)) != 1 || !g.Tem(m, Consulta) {
			t.Errorf("gerência em %s deve ter só Consulta: %v", m, g.AcoesDe(m))
		}
	}
	if !g.Tem("METAS", Inclusao) || g.Tem("METAS", Exclusao) || len(g.AcoesDe("PRODUTOS")) != 4 {
		t.Error("gerência: METAS (ver e definir) / PRODUTOS (tudo)")
	}
	if len(g.Direitos) != 20 { // 21 módulos menos PERMISSOES
		t.Errorf("gerência cobre %d módulos, esperado 20", len(g.Direitos))
	}
	v, _ := BuscarPerfil(VendedorLoja)
	if v.Tem("VENDAS_TODOS", Consulta) || v.Tem("DOCUMENTOS", Exclusao) || !v.Tem("APP_VENDAS", Inclusao) || v.Tem("BI_EXECUTIVO", Consulta) {
		t.Error("vendedor da loja errado")
	}
	s, _ := BuscarPerfil(ProfissionalSalao)
	if !s.Tem("COMISSOES", Consulta) || !s.Tem("AGENDA", Inclusao) || s.Tem("AGENDA", Exclusao) || s.Tem("AGENDA_TODOS", Consulta) || s.Tem("APP_VENDAS", Consulta) {
		t.Error("profissional do salão errado")
	}
	r, _ := BuscarPerfil(Recepcao)
	if len(r.AcoesDe("AGENDA")) != 4 || !r.Tem("AGENDA_TODOS", Consulta) || r.Tem("DOCUMENTOS", Consulta) || r.Tem("ESTOQUE", Consulta) {
		t.Error("recepção errada")
	}
	if _, ok := BuscarPerfil("INEXISTENTE"); ok {
		t.Error("perfil inexistente foi encontrado")
	}
	// todo módulo citado por um perfil existe no catálogo
	conhecidos := map[string]bool{}
	for _, m := range Modulos {
		conhecidos[m] = true
	}
	for _, p := range Perfis {
		for m := range p.Direitos {
			if !conhecidos[m] {
				t.Errorf("perfil %s cita módulo desconhecido %s", p.Codigo, m)
			}
		}
	}
}
