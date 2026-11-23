package web

import (
	"net/url"
	"strings"
	"testing"
)

func produtoForm(nome string, extra ...string) url.Values {
	v := url.Values{
		"uprNome": {nome}, "uprComplemento": {""}, "uprUnd": {"UN"}, "uprPreco": {"10.50"},
		"uprAtivo": {"true"}, "_uprAtivo": {"on"}, "_uprServico": {"on"}, "uprComissaoPerc": {""}, "uprDuracaoMin": {""},
		"segmentoId": {""}, "uprCatVenda": {""}, "uprCatCompra": {""},
	}
	for i := 0; i+1 < len(extra); i += 2 {
		switch extra[i] {
		case "unSiglas", "unFatores", "unPrecos":
			v.Add(extra[i], extra[i+1])
		default:
			v.Set(extra[i], extra[i+1])
		}
	}
	return v
}

// conta executa um SELECT count(*) no banco do teste.
func (a *ambiente) conta(q string, args ...any) int {
	a.t.Helper()
	var n int
	if err := a.banco.R.QueryRow(q, args...).Scan(&n); err != nil {
		a.t.Fatal(err)
	}
	return n
}

func TestProdutoNovoCriaALinhaDeEstoqueZerada(t *testing.T) {
	a, n := logadoAdmin(t)
	st, loc := n.postar("/produtos/salvar", produtoForm("Colar Teste", "uprPreco", "89.9", "uprComplemento", "banhado"))
	if st != 302 || loc != "/produtos" {
		t.Fatalf("salvar = %d → %s", st, loc)
	}
	n.esperaMensagem("/produtos", "Produto salvo com sucesso!")
	if a.conta(`SELECT count(*) FROM Estoque WHERE est_upr_codigo = 1 AND est_qtd = 0 AND est_valor_custo = 0`) != 1 {
		t.Error("produto novo (não serviço) deveria nascer com Estoque 0/0")
	}
	var preco int64
	a.banco.R.QueryRow(`SELECT upr_preco FROM Produto WHERE upr_codigo = 1`).Scan(&preco)
	if preco != 8990 {
		t.Errorf("preço gravado = %d centavos, esperado 8990", preco)
	}
	corpo := n.paginaOK("/produtos")
	for _, s := range []string{"Colar Teste", "banhado", "R$ 89,90", "Ativo"} {
		if !strings.Contains(corpo, s) {
			t.Errorf("lista sem %q", s)
		}
	}
	form := n.paginaOK("/produtos/editar/1")
	for _, s := range []string{`value="Colar Teste"`, `value="89.90"`, `<option value="UN" selected="selected">UN — Unidade</option>`, "Editar Produto"} {
		if !strings.Contains(form, s) {
			t.Errorf("formulário de edição sem %q", s)
		}
	}
}

func TestProdutoPrecoArredondaSoNaGravacao(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("A", "uprPreco", "12.345"))
	var preco int64
	a.banco.R.QueryRow(`SELECT upr_preco FROM Produto WHERE upr_codigo = 1`).Scan(&preco)
	if preco != 1235 { // HALF_UP em 2 casas
		t.Errorf("12,345 → %d centavos, esperado 1235", preco)
	}
	// 0,009 está abaixo do mínimo de 0,01: a validação olha o número digitado, antes de arredondar
	v := produtoForm("B", "uprPreco", "0.009")
	v.Set("_csrf", n.csrfLogado())
	resp, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
	if resp.StatusCode != 200 || !strings.Contains(corpo, `<span class="field-error">Preço deve ser maior que zero</span>`) {
		t.Errorf("0,009 deveria ser recusado (status %d)", resp.StatusCode)
	}
}

func TestProdutoValidacoesVoltamComODigitado(t *testing.T) {
	_, n := logadoAdmin(t)
	casos := []struct {
		nome  string
		extra []string
		msg   string
	}{
		{"  ", nil, "Nome do produto é obrigatório"},
		{strings.Repeat("N", 151), nil, "Nome deve ter no máximo 150 caracteres"},
		{"x", []string{"uprUnd", ""}, "Unidade é obrigatória"},
		{"x", []string{"uprPreco", ""}, "Preço é obrigatório"},
		{"x", []string{"uprPreco", "0"}, "Preço deve ser maior que zero"},
		{"x", []string{"uprPreco", "-3"}, "Preço deve ser maior que zero"},
		{"x", []string{"uprServico", "true", "uprComissaoPerc", "100.01"}, "A comissão não pode passar de 100%"},
		{"x", []string{"uprServico", "true", "uprComissaoPerc", "-0.01"}, "A comissão não pode ser negativa"},
		{"x", []string{"uprServico", "true", "uprDuracaoMin", "0"}, "A duração deve ser de pelo menos 1 minuto"},
		{"x", []string{"uprServico", "true", "uprDuracaoMin", "1441"}, "A duração não pode passar de 24 horas"},
		// a validação vale mesmo para quem não é serviço (a comissão só é zerada depois)
		{"x", []string{"uprComissaoPerc", "150"}, "A comissão não pode passar de 100%"},
	}
	for _, c := range casos {
		v := produtoForm(c.nome, append(c.extra, "uprComplemento", "mantém isto")...)
		v.Set("_csrf", n.csrfLogado())
		resp, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("%q: esperado 200 (tela com erro), veio %d", c.msg, resp.StatusCode)
		}
		if !strings.Contains(corpo, `<span class="field-error">`+c.msg+`</span>`) {
			t.Errorf("faltou %q ao lado do campo", c.msg)
		}
		if !strings.Contains(corpo, `value="mantém isto"`) || !strings.Contains(corpo, "Novo Produto") {
			t.Errorf("%q: a tela deveria voltar com o digitado e o título de produto novo", c.msg)
		}
	}
	// complemento > 255: barra sem mostrar texto, como o antigo
	v := produtoForm("x", "uprComplemento", strings.Repeat("c", 256))
	v.Set("_csrf", n.csrfLogado())
	resp, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
	if resp.StatusCode != 200 || strings.Contains(corpo, "field-error") {
		t.Errorf("complemento longo: status %d, mensagem visível %v", resp.StatusCode, strings.Contains(corpo, "field-error"))
	}
	if n.a.conta(`SELECT count(*) FROM Produto`) != 0 {
		t.Error("nada deveria ter sido gravado")
	}
}

func TestProdutoUnidadeBaseRegras(t *testing.T) {
	a, n := logadoAdmin(t)
	posta := func(extra ...string) string {
		v := produtoForm("U", extra...)
		v.Set("_csrf", n.csrfLogado())
		resp, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
		if resp.StatusCode == 302 {
			return "ok"
		}
		return corpo
	}
	if corpo := posta("uprUnd", "ZZ"); !strings.Contains(corpo, "A unidade de medida &#39;ZZ&#39; não está cadastrada.") {
		t.Error("unidade desconhecida deveria ser recusada")
	}
	if posta("uprUnd", "un") != "ok" {
		t.Fatal("unidade com outra caixa deveria ser aceita")
	}
	var und string
	a.banco.R.QueryRow(`SELECT upr_und FROM Produto WHERE upr_codigo = 1`).Scan(&und)
	if und != "UN" {
		t.Errorf("a unidade deve ficar como no cadastro: %q", und)
	}
	// unidade inativa: nova não, mas quem já usa continua podendo
	a.banco.W.Exec(`UPDATE Unidade_Medida SET und_ativo = 0 WHERE und_sigla = 'MT'`)
	if corpo := posta("uprUnd", "MT"); !strings.Contains(corpo, "A unidade de medida &#39;MT&#39; está inativa.") {
		t.Error("unidade inativa deveria ser recusada")
	}
	a.banco.W.Exec(`UPDATE Unidade_Medida SET und_ativo = 1 WHERE und_sigla = 'MT'`)
	n.postar("/produtos/salvar", produtoForm("M", "uprUnd", "MT"))
	a.banco.W.Exec(`UPDATE Unidade_Medida SET und_ativo = 0 WHERE und_sigla = 'MT'`)
	if st, _ := n.postar("/produtos/salvar", produtoForm("M2", "uprCodigo", "2", "uprUnd", "MT")); st != 302 {
		t.Error("o produto que já usa a unidade inativa deveria poder ser salvo")
	}
	if corpo := n.paginaOK("/produtos/editar/2"); !strings.Contains(corpo, `<option value="MT" selected="selected">MT — Metro</option>`) {
		t.Error("o formulário deve continuar oferecendo a unidade inativa do produto")
	}
}

func TestProdutoServicoSemEstoqueEComissao(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("Corte", "uprServico", "true", "uprComissaoPerc", "12.5", "uprDuracaoMin", "90", "uprPreco", "80"))
	if a.conta(`SELECT count(*) FROM Estoque`) != 0 {
		t.Error("serviço não tem linha de estoque")
	}
	corpo := n.paginaOK("/produtos")
	if !strings.Contains(corpo, "Serviço · 90 min · comissão 12,50%") {
		t.Errorf("selo do serviço incorreto:\n%s", trecho(corpo, "badge-blue"))
	}
	form := n.paginaOK("/produtos/editar/1")
	if !strings.Contains(form, `value="12.50"`) || !strings.Contains(form, `value="90"`) {
		t.Error("o formulário deve trazer comissão 12.50 e duração 90")
	}
	// serviço → produto: nasce a linha de estoque e a comissão e a duração são zeradas
	n.postar("/produtos/salvar", produtoForm("Corte", "uprCodigo", "1", "uprComissaoPerc", "30", "uprDuracaoMin", "20"))
	if a.conta(`SELECT count(*) FROM Estoque WHERE est_upr_codigo = 1 AND est_qtd = 0`) != 1 {
		t.Error("ao virar produto deve ganhar a linha de estoque zerada")
	}
	if a.conta(`SELECT count(*) FROM Produto WHERE upr_codigo = 1 AND upr_comissao_perc IS NULL AND upr_duracao_min IS NULL`) != 1 {
		t.Error("produto (não serviço) não guarda comissão nem duração")
	}
	// produto com saldo não vira serviço
	a.banco.W.Exec(`UPDATE Estoque SET est_qtd = 2500 WHERE est_upr_codigo = 1`)
	v := produtoForm("Corte", "uprCodigo", "1", "uprServico", "true")
	v.Set("_csrf", n.csrfLogado())
	_, corpo = n.fazer("POST", "/produtos/salvar", v, nil)
	if !strings.Contains(corpo, "Este produto ainda tem 2.5 em estoque. Zere o estoque (saída ou ajuste) antes de transformá-lo em serviço.") {
		t.Errorf("faltou o aviso de estoque:\n%s", trecho(corpo, "alert"))
	}
	if a.conta(`SELECT count(*) FROM Produto WHERE upr_codigo = 1 AND upr_servico = 0`) != 1 || a.conta(`SELECT count(*) FROM Estoque`) != 1 {
		t.Error("a recusa não pode alterar nada")
	}
	// zerado: pode, e a linha de estoque some
	a.banco.W.Exec(`UPDATE Estoque SET est_qtd = 0 WHERE est_upr_codigo = 1`)
	n.postar("/produtos/salvar", produtoForm("Corte", "uprCodigo", "1", "uprServico", "true"))
	if a.conta(`SELECT count(*) FROM Estoque`) != 0 {
		t.Error("ao virar serviço a linha de estoque deve ser removida")
	}
}

func TestProdutoUnidadesExtras(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("P"))
	erro := func(msg string, extra ...string) {
		t.Helper()
		v := produtoForm("P", append([]string{"uprCodigo", "1"}, extra...)...)
		v.Set("_csrf", n.csrfLogado())
		resp, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
		if resp.StatusCode != 200 || !strings.Contains(corpo, msg) {
			t.Errorf("esperado erro %q (status %d)\n%s", msg, resp.StatusCode, trecho(corpo, "alert"))
		}
	}
	erro("A unidade &#39;CJ&#39; está repetida.", "unSiglas", "CJ", "unFatores", "12", "unPrecos", "", "unSiglas", "cj", "unFatores", "3", "unPrecos", "")
	erro("A unidade &#39;UN&#39; já é a unidade base do produto.", "unSiglas", "un", "unFatores", "2", "unPrecos", "")
	erro("Informe quantas UN cabem em 1 CJ (maior que zero).", "unSiglas", "CJ", "unFatores", "0", "unPrecos", "")
	erro("O fator de CJ é grande demais (máximo 1.000.000).", "unSiglas", "CJ", "unFatores", "1000001", "unPrecos", "")
	erro("O preço de CJ deve ser maior que zero (ou deixe em branco).", "unSiglas", "CJ", "unFatores", "12", "unPrecos", "0")
	erro("Escolha a unidade em todas as linhas de unidades adicionais.", "unSiglas", "", "unFatores", "12", "unPrecos", "")
	erro("A unidade de medida &#39;ZZ&#39; não está cadastrada.", "unSiglas", "ZZ", "unFatores", "12", "unPrecos", "")

	n.postar("/produtos/salvar", produtoForm("P", "uprCodigo", "1", "unSiglas", "CJ", "unFatores", "12", "unPrecos", "100", "unSiglas", "kg", "unFatores", "0.5", "unPrecos", ""))
	var fator, preco int64
	a.banco.R.QueryRow(`SELECT pun_fator, pun_preco FROM Produto_Unidade WHERE pun_sigla = 'CJ'`).Scan(&fator, &preco)
	if fator != 12_000_000 || preco != 10000 {
		t.Errorf("CJ: fator %d, preço %d (esperado 12000000 e 10000)", fator, preco)
	}
	form := n.paginaOK("/produtos/editar/1")
	for _, s := range []string{`value="12.000000"`, `value="100.00"`, `value="0.500000"`, `<option value="KG" selected="selected">KG — Quilograma</option>`} {
		if !strings.Contains(form, s) {
			t.Errorf("formulário sem %q", s)
		}
	}
	// com extras no formulário a base não muda; sem elas, muda
	v := produtoForm("P", "uprCodigo", "1", "uprUnd", "MT", "unSiglas", "CJ", "unFatores", "12", "unPrecos", "")
	v.Set("_csrf", n.csrfLogado())
	_, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
	if !strings.Contains(corpo, "Para trocar a unidade base (UN), remova antes as unidades adicionais do produto") {
		t.Error("faltou o aviso de troca de base")
	}
	n.postar("/produtos/salvar", produtoForm("P", "uprCodigo", "1", "uprUnd", "MT"))
	if a.conta(`SELECT count(*) FROM Produto_Unidade`) != 0 || a.conta(`SELECT count(*) FROM Produto WHERE upr_und = 'MT'`) != 1 {
		t.Error("sem linhas, a base troca e as extras somem")
	}
}

func TestProdutoNovoQueFalhaDepoisDeGravarQueimaOCodigoEMostraEditar(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("Primeiro"))
	v := produtoForm("Segundo", "unSiglas", "ZZ", "unFatores", "2", "unPrecos", "")
	v.Set("_csrf", n.csrfLogado())
	resp, corpo := n.fazer("POST", "/produtos/salvar", v, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	// como no antigo: o formulário volta como "Editar Produto" com o código que foi gerado e desfeito
	if !strings.Contains(corpo, "<span>Editar Produto</span>") || !strings.Contains(corpo, `id="uprCodigo" name="uprCodigo" value="2"`) {
		t.Errorf("faltou o título e o código do antigo:\n%s", trecho(corpo, "page-title"))
	}
	if a.conta(`SELECT count(*) FROM Produto`) != 1 {
		t.Error("o produto que falhou não pode ficar gravado")
	}
	n.postar("/produtos/salvar", produtoForm("Terceiro"))
	if a.conta(`SELECT count(*) FROM Produto WHERE upr_codigo = 3 AND upr_nome = 'Terceiro'`) != 1 {
		t.Error("o código 2 fica queimado: o próximo produto é o 3 (como a coluna IDENTITY do SQL Server)")
	}
}

func TestProdutoStatusEBuscaSoComAtivos(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("Anel Solitário"))
	n.postar("/produtos/salvar", produtoForm("Brinco Pérola"))
	n.postar("/produtos/status/2", url.Values{"ativo": {"false"}})
	n.esperaMensagem("/produtos", "Status do produto atualizado!")
	corpo := n.paginaOK("/produtos")
	if !strings.Contains(corpo, "Brinco Pérola") || !strings.Contains(corpo, `<tr class="row-inactive">`) {
		t.Error("o inativo aparece na lista completa, com destaque")
	}
	if strings.Contains(n.paginaOK("/produtos?busca=perola"), "Brinco Pérola") {
		t.Error("a busca por texto só traz produtos ativos (RN-PROD-06)")
	}
	if !strings.Contains(n.paginaOK("/produtos?busca=SOLITARIO"), "Anel Solitário") {
		t.Error("a busca ignora acento e caixa")
	}
	// ativar exige preço de pelo menos 0,01 (produtos vindos de carga com preço zero)
	a.banco.W.Exec(`UPDATE Produto SET upr_preco = 0 WHERE upr_codigo = 2`)
	st, loc := n.postar("/produtos/status/2", url.Values{"ativo": {"true"}})
	if st != 302 || loc != "/produtos/editar/2" {
		t.Fatalf("ativar sem preço = %d → %s", st, loc)
	}
	n.esperaMensagem("/produtos/editar/2", "Defina o preço do produto &#34;Brinco Pérola&#34; (maior que zero) antes de ativar: clique em Editar.")
	if a.conta(`SELECT count(*) FROM Produto WHERE upr_codigo = 2 AND upr_ativo = 0`) != 1 {
		t.Error("o produto deveria continuar inativo")
	}
	// inativar sempre funciona; inexistente não reclama
	n.postar("/produtos/status/1", url.Values{"ativo": {"false"}})
	n.postar("/produtos/status/999", url.Values{"ativo": {"true"}})
	n.esperaMensagem("/produtos", "Status do produto atualizado!")
}

func TestProdutoBuscaTrataPorcentoEUnderlineComoLetras(t *testing.T) {
	_, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("Desconto 10% off"))
	n.postar("/produtos/salvar", produtoForm("Outro produto"))
	corpo := n.paginaOK("/produtos?busca=%25")
	if !strings.Contains(corpo, "Desconto 10% off") || strings.Contains(corpo, "Outro produto") {
		t.Error("% na busca deve valer como o caractere %, não como coringa")
	}
	if strings.Contains(n.paginaOK("/produtos?busca=_"), "Outro produto") {
		t.Error("_ na busca deve valer como o caractere _")
	}
}

func TestProdutoExcluirCorrigidoD03(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/produtos/salvar", produtoForm("Sem documento", "unSiglas", "CJ", "unFatores", "12", "unPrecos", ""))
	n.postar("/produtos/salvar", produtoForm("Com documento"))
	n.postar("/produtos/salvar", produtoForm("Com saldo"))
	uid := a.criarUsuario("op", "Operador", "Operador-Senha-1", true)
	a.banco.W.Exec(`INSERT INTO Pessoas (ups_nome, ups_fornecedor) VALUES ('Forn', 1)`)
	a.banco.W.Exec(`INSERT INTO Documento (docto_tipo, docto_data, docto_valor, docto_ups_codigo, docto_usu_codigo) VALUES ('COMPRA','2026-11-20',100,1,?)`, uid)
	a.banco.W.Exec(`INSERT INTO Itens_Documento (itemdocto_docto_codigo, itemdocto_upr_codigo, itemdocto_upr_und, itemdocto_qtd, itemdocto_valor, itemdocto_total) VALUES (1,2,'UN',1000,100,100)`)
	a.banco.W.Exec(`UPDATE Estoque SET est_qtd = 5000 WHERE est_upr_codigo = 3`)

	n.postar("/produtos/excluir/1", nil)
	n.esperaMensagem("/produtos", "Produto excluído com sucesso!")
	if a.conta(`SELECT count(*) FROM Produto WHERE upr_codigo = 1`) != 0 || a.conta(`SELECT count(*) FROM Estoque WHERE est_upr_codigo = 1`) != 0 || a.conta(`SELECT count(*) FROM Produto_Unidade WHERE pun_upr_codigo = 1`) != 0 {
		t.Error("deveria apagar o produto, a linha de estoque zerada e as unidades extras")
	}
	for _, id := range []string{"2", "3"} {
		n.postar("/produtos/excluir/"+id, nil)
		n.esperaMensagem("/produtos", "Não foi possível excluir o produto. Verifique se há documentos vinculados.")
	}
	if a.conta(`SELECT count(*) FROM Produto WHERE upr_codigo IN (2,3)`) != 2 || a.conta(`SELECT count(*) FROM Estoque WHERE est_upr_codigo IN (2,3)`) != 2 {
		t.Error("produto com documento ou com saldo não pode perder nada")
	}
	n.postar("/produtos/excluir/999", nil)
	n.esperaMensagem("/produtos", "Produto excluído com sucesso!") // como o deleteById do antigo, inexistente não reclama
}

func TestProdutoEditarInexistenteEParametrosRuins(t *testing.T) {
	_, n := logadoAdmin(t)
	n.get("/produtos/editar/999")
	n.esperaMensagem("/produtos", "Produto não encontrado.")
	if resp, _ := n.get("/produtos/editar/abc"); resp.StatusCode != 400 {
		t.Errorf("id não numérico = %d, esperado 400", resp.StatusCode)
	}
	if st, _ := n.postar("/produtos/status/1", url.Values{"ativo": {"talvez"}}); st != 400 {
		t.Errorf("booleano ruim = %d, esperado 400", st)
	}
	v := produtoForm("x", "unSiglas", "CJ", "unFatores", "abc", "unPrecos", "")
	v.Set("_csrf", n.csrfLogado())
	if resp, _ := n.fazer("POST", "/produtos/salvar", v, nil); resp.StatusCode != 400 {
		t.Errorf("fator não numérico = %d, esperado 400 (como o Spring)", resp.StatusCode)
	}
}

func TestProdutosRespeitamAsPermissoes(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.banco.W.Exec(`INSERT INTO Produto (upr_nome, upr_und, upr_preco) VALUES ('Alguém', 'UN', 100)`)
	a.criarUsuario("leitor", "Leitor", "Leitor-Senha-1", true, "PRODUTOS:C")
	l := a.novoNavegador()
	l.entrar("leitor", "Leitor-Senha-1")
	corpo := l.paginaOK("/produtos")
	for _, nao := range []string{"Novo Produto", "/produtos/editar/", "/produtos/excluir/", "/produtos/status/"} {
		if strings.Contains(corpo, nao) {
			t.Errorf("quem só consulta não deveria ver %q", nao)
		}
	}
	l.postar("/produtos/excluir/1", nil)
	l.esperaMensagem("/dashboard", "Você não tem permissão para excluir registros de Produtos.")
	l.postar("/produtos/salvar", produtoForm("x", "uprCodigo", "1"))
	l.esperaMensagem("/dashboard", "Você não tem permissão para alterar registros em Produtos.")
	l.postar("/produtos/salvar", produtoForm("x"))
	l.esperaMensagem("/dashboard", "Você não tem permissão para incluir registros em Produtos.")
	// a tela do formulário mostra o atalho para Unidades só a quem pode consultá-las
	a.criarUsuario("editor", "Editor", "Editor-Senha-1", true, "PRODUTOS:CIA")
	e := a.novoNavegador()
	e.entrar("editor", "Editor-Senha-1")
	if strings.Contains(e.paginaOK("/produtos/novo"), "Cadastre em Unidades de medida") {
		t.Error("sem Consulta em Unidades o atalho não aparece")
	}
	if !strings.Contains(a.admin(t).paginaOK("/produtos/novo"), "Cadastre em Unidades de medida") {
		t.Error("o admin vê o atalho para Unidades")
	}
}

func (a *ambiente) admin(t *testing.T) *navegador {
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	return n
}

// ───────────────────────────── formatação de números ─────────────────────────────

func TestFormatoDeNumeros(t *testing.T) {
	escalas := []struct {
		v     int64
		casas int
		quer  string
	}{{8990, 2, "89.90"}, {5, 2, "0.05"}, {0, 2, "0.00"}, {12_000_000, 6, "12.000000"}, {500_000, 6, "0.500000"}, {7, 0, "7"}, {-150, 2, "-1.50"}}
	for _, c := range escalas {
		if got := escala(c.v, c.casas); got != c.quer {
			t.Errorf("escala(%d,%d) = %q, esperado %q", c.v, c.casas, got, c.quer)
		}
	}
	br := []struct {
		v     int64
		casas int
		quer  string
	}{{8990, 2, "89,90"}, {123456, 2, "1.234,56"}, {100000000, 2, "1.000.000,00"}, {5, 2, "0,05"}, {4000, 2, "40,00"}}
	for _, c := range br {
		if got := decimalBR(c.v, c.casas); got != c.quer {
			t.Errorf("decimalBR(%d,%d) = %q, esperado %q", c.v, c.casas, got, c.quer)
		}
	}
	if moeda(123456) != "R$ 1.234,56" {
		t.Errorf("moeda = %q", moeda(123456))
	}
	digitados := map[string]string{"89.90": "89.90", "089.9": "89.9", ".5": "0.5", "+7": "7", "5.": "5", "12": "12", "-0.50": "-0.50"}
	for in, quer := range digitados {
		if got, ok := textoBigDecimal(in); !ok || got != quer {
			t.Errorf("textoBigDecimal(%q) = %q,%v, esperado %q", in, got, ok, quer)
		}
	}
	for _, ruim := range []string{"", "abc", "1,5", "--1", "."} {
		if _, ok := textoBigDecimal(ruim); ok {
			t.Errorf("textoBigDecimal(%q) deveria falhar", ruim)
		}
	}
}
