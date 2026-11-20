package web

import (
	"net/url"
	"strings"
	"testing"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
)

// postar envia um formulário com o token CSRF da sessão e devolve a resposta.
func (n *navegador) postar(caminho string, v url.Values) (status int, local string) {
	n.a.t.Helper()
	if v == nil {
		v = url.Values{}
	}
	v.Set("_csrf", n.csrfLogado())
	resp, _ := n.fazer("POST", caminho, v, nil)
	return resp.StatusCode, resp.Header.Get("Location")
}

func (n *navegador) paginaOK(caminho string) string {
	n.a.t.Helper()
	resp, corpo := n.get(caminho)
	if resp.StatusCode != 200 {
		n.a.t.Fatalf("GET %s = %d → %s", caminho, resp.StatusCode, resp.Header.Get("Location"))
	}
	return corpo
}

func (n *navegador) esperaMensagem(caminho, mensagem string) {
	n.a.t.Helper()
	corpo := n.paginaOK(caminho)
	if !strings.Contains(corpo, mensagem) {
		n.a.t.Fatalf("a tela %s não mostra %q\n%s", caminho, mensagem, trecho(corpo, "alert"))
	}
}

func logadoAdmin(t *testing.T) (*ambiente, *navegador) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	return a, n
}

func unidadeForm(sigla, desc string, extra ...string) url.Values {
	v := url.Values{"undSigla": {sigla}, "undDescricao": {desc}, "undAtivo": {"true"}, "_undAtivo": {"on"}}
	for i := 0; i+1 < len(extra); i += 2 {
		v.Set(extra[i], extra[i+1])
	}
	return v
}

// ───────────────────────────── Unidades de medida ─────────────────────────────

func TestUnidadesListaTrazOsDadosIniciais(t *testing.T) {
	_, n := logadoAdmin(t)
	corpo := n.paginaOK("/unidades")
	for _, s := range []string{"Unidades de medida", "Unidade", "Conjunto", "Quilograma", "Nova unidade", `<td class="fw-500">UN</td>`} {
		if !strings.Contains(corpo, s) {
			t.Errorf("lista de unidades sem %q", s)
		}
	}
	if !strings.Contains(corpo, `<li class="active">`) || !strings.Contains(corpo, `href="/unidades"`) {
		t.Error("o menu deveria destacar Unidades")
	}
}

func TestUnidadeCriaEmMaiusculasEFlash(t *testing.T) {
	_, n := logadoAdmin(t)
	st, loc := n.postar("/unidades/salvar", unidadeForm("cx2", "  Caixa dois  "))
	if st != 302 || loc != "/unidades" {
		t.Fatalf("salvar = %d → %s", st, loc)
	}
	n.esperaMensagem("/unidades", "Unidade de medida salva!")
	corpo := n.paginaOK("/unidades")
	if !strings.Contains(corpo, `<td class="fw-500">CX2</td>`) || !strings.Contains(corpo, "<td>Caixa dois</td>") {
		t.Error("a unidade deveria estar em maiúsculas e com a descrição sem espaços nas pontas")
	}
	if strings.Contains(corpo, "Unidade de medida salva!") {
		t.Error("a mensagem deve aparecer uma vez só")
	}
}

func TestUnidadeValidacoesComAsMensagensDoAntigo(t *testing.T) {
	_, n := logadoAdmin(t)
	n.postar("/unidades/salvar", unidadeForm("CX2", "Caixa"))
	casos := []struct{ sigla, desc, msg string }{
		{"  ", "x", "Informe a sigla da unidade."},
		{"ABCDEFGHIJK", "x", "A sigla pode ter no máximo 10 caracteres."},
		{"A B", "x", "A sigla não pode ter espaços."},
		{"ZZ", "  ", "Informe a descrição da unidade."},
		{"ZZ", strings.Repeat("x", 61), "A descrição pode ter no máximo 60 caracteres."},
		{"cx2", "outra", "Já existe a unidade &#39;CX2&#39;."},
	}
	for _, c := range casos {
		st, loc := n.postar("/unidades/salvar", unidadeForm(c.sigla, c.desc))
		if st != 302 || loc != "/unidades/novo" {
			t.Fatalf("%q: %d → %s", c.sigla, st, loc)
		}
		n.esperaMensagem("/unidades/novo", c.msg)
	}
}

func TestUnidadeEmUsoNaoTrocaSiglaNemExclui(t *testing.T) {
	a, n := logadoAdmin(t)
	a.banco.W.Exec(`INSERT INTO Produto (upr_nome, upr_und, upr_preco) VALUES ('A','UN',100), ('B','un',100)`)
	// UN é a unidade 1 do cadastro inicial
	st, loc := n.postar("/unidades/salvar", unidadeForm("UNI", "Unidade", "undCodigo", "1"))
	if st != 302 || loc != "/unidades/editar/1" {
		t.Fatalf("trocar sigla em uso = %d → %s", st, loc)
	}
	n.esperaMensagem("/unidades/editar/1", "A sigla &#39;UN&#39; é usada por 2 produto(s) e não pode ser trocada. Altere só a descrição ou crie uma unidade nova.")
	if corpo := n.paginaOK("/unidades/editar/1"); !strings.Contains(corpo, `readonly="readonly"`) || !strings.Contains(corpo, "Usada por 2 produto(s)") {
		t.Error("o formulário deveria travar a sigla e explicar o motivo")
	}
	// só a descrição muda
	n.postar("/unidades/salvar", unidadeForm("un", "Unidade simples", "undCodigo", "1"))
	n.esperaMensagem("/unidades", "Unidade de medida salva!")
	if !strings.Contains(n.paginaOK("/unidades"), "Unidade simples") {
		t.Error("a descrição deveria ter mudado")
	}
	// excluir
	n.postar("/unidades/excluir/1", nil)
	n.esperaMensagem("/unidades", "Não é possível excluir &#39;UN&#39;: 2 produto(s) usam esta unidade. Inative-a para ela deixar de ser oferecida.")
}

func TestUnidadeStatusEExclusao(t *testing.T) {
	_, n := logadoAdmin(t)
	n.postar("/unidades/salvar", unidadeForm("TMP", "Temporária"))
	n.postar("/unidades/status/6", url.Values{"ativo": {"false"}})
	corpo := n.paginaOK("/unidades")
	if !strings.Contains(corpo, "Status atualizado!") || !strings.Contains(corpo, `<tr class="row-inactive">`) || !strings.Contains(corpo, "Inativa") {
		t.Error("a unidade deveria aparecer inativa")
	}
	n.postar("/unidades/status/6", url.Values{"ativo": {"true"}})
	if strings.Contains(n.paginaOK("/unidades"), "row-inactive") {
		t.Error("a unidade deveria voltar a ativa")
	}
	// a caixa desmarcada (só o marcador) cria unidade inativa
	n.postar("/unidades/salvar", url.Values{"undSigla": {"ina"}, "undDescricao": {"Inativa"}, "_undAtivo": {"on"}})
	if !strings.Contains(n.paginaOK("/unidades"), "row-inactive") {
		t.Error("sem a caixa marcada a unidade deve nascer inativa")
	}
	n.postar("/unidades/excluir/6", nil)
	n.esperaMensagem("/unidades", "Unidade de medida excluída!")
	n.postar("/unidades/excluir/999", nil)
	n.esperaMensagem("/unidades", "Unidade de medida não encontrada.")
	// parâmetros ruins = 400
	if st, _ := n.postar("/unidades/status/6", url.Values{"ativo": {"talvez"}}); st != 400 {
		t.Errorf("valor booleano ruim = %d, esperado 400", st)
	}
	if st, _ := n.postar("/unidades/status/6", nil); st != 400 {
		t.Errorf("sem o parâmetro = %d, esperado 400", st)
	}
	if resp, _ := n.get("/unidades/editar/abc"); resp.StatusCode != 400 {
		t.Errorf("id não numérico = %d, esperado 400", resp.StatusCode)
	}
	n.get("/unidades/editar/999")
	n.esperaMensagem("/unidades", "Unidade de medida não encontrada.")
}

func TestUnidadesRespeitamAsPermissoes(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("leitor", "Leitor", "Leitor-Senha-1", true, "UNIDADES:C")
	n := a.novoNavegador()
	n.entrar("leitor", "Leitor-Senha-1")
	corpo := n.paginaOK("/unidades")
	for _, nao := range []string{"Nova unidade", "/unidades/editar/", "/unidades/excluir/", "/unidades/status/"} {
		if strings.Contains(corpo, nao) {
			t.Errorf("quem só consulta não deveria ver %q", nao)
		}
	}
	resp, _ := n.get("/unidades/novo")
	deve(t, resp, 302, "/dashboard")
	if st, loc := n.postar("/unidades/salvar", unidadeForm("X", "x")); st != 302 || loc != "/dashboard" {
		t.Errorf("incluir sem direito = %d → %s", st, loc)
	}
	n.esperaMensagem("/dashboard", "Você não tem permissão para incluir registros em Unidades de medida.")
	n.postar("/unidades/excluir/1", nil)
	n.esperaMensagem("/dashboard", "Você não tem permissão para excluir registros de Unidades de medida.")
}

// ───────────────────────────── Pessoas ─────────────────────────────

func pessoaForm(nome string, extra ...string) url.Values {
	v := url.Values{
		"upsNome": {nome}, "upsCpf": {""}, "upsTelefone": {""}, "upsNascimento": {""},
		"upsCliente": {"true"}, "_upsCliente": {"on"}, "_upsFornecedor": {"on"}, "_upsVendedor": {"on"}, "_upsFuncionario": {"on"},
		"upsAtivo": {"true"}, "_upsAtivo": {"on"},
	}
	for i := 0; i+1 < len(extra); i += 2 {
		if extra[i] == "segmentoIds" {
			v.Add("segmentoIds", extra[i+1])
		} else {
			v.Set(extra[i], extra[i+1])
		}
	}
	return v
}

func TestPessoaCriaListaEEdita(t *testing.T) {
	_, n := logadoAdmin(t)
	st, loc := n.postar("/pessoas/salvar", pessoaForm("João da Silva", "upsCpf", "123.456.789-00", "upsTelefone", "(17) 99999-0001", "upsNascimento", "1990-05-10", "segmentoIds", "1", "segmentoIds", "2"))
	if st != 302 || loc != "/pessoas" {
		t.Fatalf("salvar = %d → %s", st, loc)
	}
	corpo := n.paginaOK("/pessoas")
	for _, s := range []string{"Pessoa salva com sucesso!", "João da Silva", "123.456.789-00", "(17) 99999-0001", "10/05/1990", "Cliente", "Loja de roupa", "Salão", "Ativo"} {
		if !strings.Contains(corpo, s) {
			t.Errorf("lista sem %q", s)
		}
	}
	if strings.Contains(corpo, `<li class="active">`) && strings.Contains(corpo, `class="active">
            <a href="/pessoas"`) {
		t.Error("o antigo não destaca Pessoas no menu (paridade)")
	}
	// editar traz o formulário preenchido e os segmentos marcados
	form := n.paginaOK("/pessoas/editar/1")
	for _, s := range []string{`value="João da Silva"`, `value="1990-05-10"`, `name="segmentoIds" value="1" checked="checked"`, `name="segmentoIds" value="2" checked="checked"`, "Editar Pessoa"} {
		if !strings.Contains(form, s) {
			t.Errorf("formulário de edição sem %q", s)
		}
	}
	// troca de segmentos e de tipo
	n.postar("/pessoas/salvar", pessoaForm("João Jr", "upsCodigo", "1", "upsCpf", "123.456.789-00", "upsVendedor", "true", "segmentoIds", "2"))
	corpo = n.paginaOK("/pessoas")
	if !strings.Contains(corpo, "João Jr") || !strings.Contains(corpo, "Vendedor") || strings.Count(corpo, `<span class="badge badge-green">Loja de roupa</span>`) != 0 {
		t.Error("a edição deveria trocar nome, tipo e segmentos")
	}
}

func TestPessoaCPFUnicoSoPelosDigitos(t *testing.T) {
	_, n := logadoAdmin(t)
	n.postar("/pessoas/salvar", pessoaForm("Ana", "upsCpf", "123.456.789-00"))
	n.postar("/pessoas/salvar", pessoaForm("Outra", "upsCpf", "12345678900"))
	n.esperaMensagem("/pessoas", "Erro: Já existe uma pessoa cadastrada com este CPF/CNPJ.")
	if strings.Contains(n.paginaOK("/pessoas"), "Outra") {
		t.Error("a pessoa com CPF repetido não deveria ter sido gravada")
	}
	// editar mantendo o próprio CPF (mesmo escrito de outro jeito) é permitido
	n.postar("/pessoas/salvar", pessoaForm("Ana Maria", "upsCodigo", "1", "upsCpf", "123 456 789 00"))
	n.esperaMensagem("/pessoas", "Pessoa salva com sucesso!")
	// telefone repetido só avisa (nada bloqueia)
	n.postar("/pessoas/salvar", pessoaForm("Bia", "upsTelefone", "(17) 99999-0001"))
	n.postar("/pessoas/salvar", pessoaForm("Cia", "upsTelefone", "17 99999 0001"))
	corpo := n.paginaOK("/pessoas")
	if !strings.Contains(corpo, "Bia") || !strings.Contains(corpo, "Cia") {
		t.Error("telefone repetido não deve bloquear o cadastro")
	}
}

func TestPessoaValidacaoVoltaAoFormularioComOQueFoiDigitado(t *testing.T) {
	_, n := logadoAdmin(t)
	casos := []struct {
		nome  string
		extra []string
		msg   string
		campo string
		valor string
	}{
		{"   ", []string{"upsTelefone", "(17) 1"}, "Nome é obrigatório", "upsTelefone", "(17) 1"},
		{strings.Repeat("N", 151), nil, "Nome deve ter no máximo 150 caracteres", "", ""},
		{"Futuro", []string{"upsNascimento", "2099-01-01"}, "Data de nascimento não pode ser futura", "upsNascimento", "2099-01-01"},
	}
	for _, c := range casos {
		v := pessoaForm(c.nome, c.extra...)
		v.Set("_csrf", n.csrfLogado())
		resp, corpo := n.fazer("POST", "/pessoas/salvar", v, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("%q: esperado 200 (tela com erro), veio %d", c.msg, resp.StatusCode)
		}
		if !strings.Contains(corpo, `<span class="field-error">`+c.msg+`</span>`) {
			t.Errorf("faltou a mensagem %q ao lado do campo", c.msg)
		}
		if c.campo != "" && !strings.Contains(corpo, `name="`+c.campo+`" value="`+c.valor+`"`) {
			t.Errorf("o campo %s deveria manter %q", c.campo, c.valor)
		}
		if !strings.Contains(corpo, "<span>Pessoa</span>") {
			t.Error("o título da tela de erro é 'Pessoa'")
		}
	}
	// CPF com mais de 20 caracteres também barra, mas o antigo não mostra texto
	v := pessoaForm("CPF longo", "upsCpf", strings.Repeat("1", 21))
	v.Set("_csrf", n.csrfLogado())
	resp, corpo := n.fazer("POST", "/pessoas/salvar", v, nil)
	if resp.StatusCode != 200 || strings.Contains(corpo, "field-error") {
		t.Errorf("CPF longo: status %d, mensagem visível = %v", resp.StatusCode, strings.Contains(corpo, "field-error"))
	}
	// nada disso foi gravado
	var qtd int
	n.a.banco.R.QueryRow(`SELECT count(*) FROM Pessoas`).Scan(&qtd)
	if qtd != 0 {
		t.Errorf("%d pessoas gravadas apesar dos erros", qtd)
	}
}

func TestPessoaSegmentos(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/pessoas/salvar", pessoaForm("Com seg", "segmentoIds", "99"))
	n.esperaMensagem("/pessoas", "Erro: Segmento não encontrado.")
	// segmento inativo: só aceito se a pessoa já tinha
	n.postar("/pessoas/salvar", pessoaForm("Dona", "segmentoIds", "2"))
	a.banco.W.Exec(`UPDATE Segmento SET seg_ativo = 0 WHERE seg_codigo = 2`)
	n.postar("/pessoas/salvar", pessoaForm("Nova", "segmentoIds", "2"))
	n.esperaMensagem("/pessoas", "Erro: O segmento &#34;Salão&#34; está inativo.")
	n.postar("/pessoas/salvar", pessoaForm("Dona Editada", "upsCodigo", "1", "segmentoIds", "2"))
	n.esperaMensagem("/pessoas", "Pessoa salva com sucesso!")
	// o formulário só oferece segmentos ativos
	if strings.Contains(n.paginaOK("/pessoas/nova"), `value="2"`) {
		t.Error("o formulário não deveria oferecer o segmento inativo")
	}
}

func TestPessoaBuscaEFiltros(t *testing.T) {
	a, n := logadoAdmin(t)
	a.banco.W.Exec(`INSERT INTO Pessoas (ups_nome, ups_cpf, ups_telefone, ups_cliente, ups_fornecedor, ups_vendedor, ups_funcionario) VALUES
		('João Souza','111.222.333-44','(17) 99999-0001',1,0,0,0),
		('Joias Premium','12.345.678/0001-90','(11) 3333-4444',0,1,0,0),
		('Carlos Vendedor','567.890.123-44','(11) 98888-1111',0,0,1,1)`)
	a.banco.W.Exec(`INSERT INTO Pessoa_Segmento VALUES (1,2), (3,1)`)
	tem := func(caminho string, quer ...string) {
		t.Helper()
		corpo := n.paginaOK(caminho)
		for _, q := range quer {
			if !strings.Contains(corpo, q) {
				t.Errorf("%s deveria mostrar %q", caminho, q)
			}
		}
	}
	nao := func(caminho string, nao ...string) {
		t.Helper()
		corpo := n.paginaOK(caminho)
		for _, q := range nao {
			if strings.Contains(corpo, q) {
				t.Errorf("%s não deveria mostrar %q", caminho, q)
			}
		}
	}
	tem("/pessoas?busca=JOAO", "João Souza")
	nao("/pessoas?busca=JOAO", "Carlos Vendedor")
	tem("/pessoas?busca=333-44", "João Souza") // CPF parcial (a máscara conta)
	tem("/pessoas?busca=98888", "Carlos Vendedor")
	tem("/pessoas?busca=%20%20", "João Souza", "Joias Premium", "Carlos Vendedor") // em branco = todos
	nao("/pessoas?tipo=fornecedor", "João Souza")
	tem("/pessoas?tipo=fornecedor", "Joias Premium")
	tem("/pessoas?tipo=funcionario", "Carlos Vendedor")
	tem("/pessoas?tipo=xyz", "João Souza", "Joias Premium")
	tem("/pessoas?segmento=2", "João Souza")
	nao("/pessoas?segmento=2", "Carlos Vendedor")
	tem("/pessoas?segmento=1&tipo=vendedor", "Carlos Vendedor")
	tem("/pessoas?busca=joias&tipo=fornecedor", `<option value="fornecedor" selected="selected">`)
	if resp, _ := n.get("/pessoas?segmento=abc"); resp.StatusCode != 400 {
		t.Errorf("segmento não numérico = %d, esperado 400", resp.StatusCode)
	}
}

func TestPessoaExcluir(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/pessoas/salvar", pessoaForm("Sem vínculo", "segmentoIds", "1"))
	n.postar("/pessoas/salvar", pessoaForm("Fornecedora", "upsFornecedor", "true"))
	n.postar("/pessoas/excluir/1", nil)
	n.esperaMensagem("/pessoas", "Pessoa excluída!")
	var seg int
	a.banco.R.QueryRow(`SELECT count(*) FROM Pessoa_Segmento WHERE ps_ups_codigo = 1`).Scan(&seg)
	if seg != 0 {
		t.Error("os vínculos de segmento deveriam ter sido apagados junto")
	}
	// com documento vinculado: não exclui e nada é apagado
	uid := a.criarUsuario("op", "Operador", "Operador-Senha-1", true)
	a.banco.W.Exec(`INSERT INTO Documento (docto_tipo, docto_data, docto_valor, docto_ups_codigo, docto_usu_codigo) VALUES ('COMPRA','2026-11-20',100,2,?)`, uid)
	n.postar("/pessoas/excluir/2", nil)
	n.esperaMensagem("/pessoas", "Não foi possível excluir. Verifique documentos vinculados.")
	if !strings.Contains(n.paginaOK("/pessoas"), "Fornecedora") {
		t.Error("a pessoa com documento deveria continuar cadastrada")
	}
	// inexistente: o antigo não reclama
	n.postar("/pessoas/excluir/999", nil)
	n.esperaMensagem("/pessoas", "Pessoa excluída!")
	n.get("/pessoas/editar/999")
	n.esperaMensagem("/pessoas", "Pessoa não encontrada.")
}

func TestPessoasRespeitamAsPermissoes(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.banco.W.Exec(`INSERT INTO Pessoas (ups_nome) VALUES ('Alguém')`)
	a.criarUsuario("leitor", "Leitor", "Leitor-Senha-1", true, "PESSOAS:C")
	a.criarUsuario("editor", "Editor", "Editor-Senha-1", true, "PESSOAS:CIA")
	l := a.novoNavegador()
	l.entrar("leitor", "Leitor-Senha-1")
	corpo := l.paginaOK("/pessoas")
	for _, nao := range []string{"Nova Pessoa", "/pessoas/editar/", "/pessoas/excluir/"} {
		if strings.Contains(corpo, nao) {
			t.Errorf("quem só consulta não deveria ver %q", nao)
		}
	}
	e := a.novoNavegador()
	e.entrar("editor", "Editor-Senha-1")
	corpo = e.paginaOK("/pessoas")
	if !strings.Contains(corpo, "Nova Pessoa") || !strings.Contains(corpo, "/pessoas/editar/1") || strings.Contains(corpo, "/pessoas/excluir/") {
		t.Error("o editor vê incluir e editar, mas não excluir")
	}
	e.postar("/pessoas/excluir/1", nil)
	e.esperaMensagem("/dashboard", "Você não tem permissão para excluir registros de Pessoas.")
	// o "salvar" com código exige Atualização; sem código, Inclusão
	l.postar("/pessoas/salvar", pessoaForm("X", "upsCodigo", "1"))
	l.esperaMensagem("/dashboard", "Você não tem permissão para alterar registros em Pessoas.")
}

// ───────────────────────────── regras puras ─────────────────────────────

func TestDigitos(t *testing.T) {
	casos := map[string]string{"123.456.789-00": "12345678900", "12.345.678/0001-90": "12345678000190", "(17) 99999-0001": "17999990001", "": "", "abc": ""}
	for in, quer := range casos {
		if got := cadastro.Digitos(in); got != quer {
			t.Errorf("Digitos(%q) = %q, esperado %q", in, got, quer)
		}
	}
}

func TestValidarPessoaUnitario(t *testing.T) {
	ok := &cadastro.Pessoa{Nome: "Ana"}
	if e := cadastro.ValidarPessoa(ok, "2026-11-20"); len(e) != 0 {
		t.Errorf("pessoa válida deu erros: %v", e)
	}
	if e := cadastro.ValidarPessoa(&cadastro.Pessoa{Nome: "Ana", Nascimento: "2026-11-20"}, "2026-11-20"); len(e) != 0 {
		t.Error("nascer hoje é permitido (@PastOrPresent)")
	}
	if e := cadastro.ValidarPessoa(&cadastro.Pessoa{Nome: "Ana", Nascimento: "2026-11-21"}, "2026-11-20"); len(e) != 1 {
		t.Error("nascer amanhã é futuro")
	}
	if !cadastro.ValidarNascimento("") || !cadastro.ValidarNascimento("1990-02-28") || cadastro.ValidarNascimento("1990-02-30") || cadastro.ValidarNascimento("10/05/1990") {
		t.Error("ValidarNascimento incorreto")
	}
}
