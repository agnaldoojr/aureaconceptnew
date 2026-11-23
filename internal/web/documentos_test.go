package web

import (
	"net/url"
	"strings"
	"testing"
)

// docForm monta o formulário de documento. itens: "produto:qtd:valor" (unidade e profissional em branco).
func docForm(tipo string, campos map[string]string, itens ...string) url.Values {
	v := url.Values{"doctoCodigo": {""}, "doctoTipo": {tipo}, "doctoData": {"2026-11-15"}, "pessoa.upsCodigo": {"1"}, "segmentoId": {""}, "vendedorId": {""}}
	for k, val := range campos {
		v.Set(k, val)
	}
	for _, it := range itens {
		p := strings.Split(it, ":")
		v.Add("produtoIds", p[0])
		v.Add("quantidades", p[1])
		v.Add("valores", p[2])
		v.Add("unidades", "")
		v.Add("profissionalIds", "")
	}
	return v
}

// montarLoja prepara o banco de um teste: pessoas, produtos (com estoque zerado) e o admin logado.
func montarLoja(t *testing.T) (*ambiente, *navegador) {
	a, n := logadoAdmin(t)
	a.banco.W.Exec(`INSERT INTO Pessoas (ups_nome, ups_cliente, ups_fornecedor, ups_vendedor) VALUES ('Cliente',1,0,0), ('Fornecedor',0,1,0), ('Vendedor',0,0,1)`)
	for _, nome := range []string{"Produto A", "Produto B"} {
		n.postar("/produtos/salvar", produtoForm(nome, "uprPreco", "10"))
	}
	n.drenar()
	return a, n
}

func (a *ambiente) saldo(produto int) (qtd, custo int64) {
	a.t.Helper()
	if err := a.banco.R.QueryRow(`SELECT est_qtd, est_valor_custo FROM Estoque WHERE est_upr_codigo = ?`, produto).Scan(&qtd, &custo); err != nil {
		a.t.Fatal(err)
	}
	return
}

func (n *navegador) lancar(v url.Values) (loc string) {
	n.a.t.Helper()
	n.drenar()
	v.Set("_csrf", n.csrfLogado())
	resp, _ := n.fazer("POST", "/documentos/salvar", v, nil)
	return resp.Header.Get("Location")
}

func TestEntradaSomaAoEstoqueECustoMedioPonderado(t *testing.T) {
	a, n := montarLoja(t)
	if loc := n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:10:35")); loc != "/documentos" {
		t.Fatalf("entrada = %q", loc)
	}
	n.esperaMensagem("/documentos", "Documento lançado com sucesso!")
	if q, c := a.saldo(1); q != 10000 || c != 350000 {
		t.Fatalf("saldo %d, custo %d (esperado 10000 e 350000)", q, c)
	}
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:5:40"))
	if q, c := a.saldo(1); q != 15000 || c != 366667 { // (10×35 + 5×40) ÷ 15 = 36,6667
		t.Errorf("custo médio: saldo %d, custo %d (esperado 15000 e 366667)", q, c)
	}
	// a saída tira do saldo e NÃO mexe no custo
	n.lancar(docForm("SAIDA", map[string]string{"doctoClassificacao": "QUEBRA"}, "1:3:40"))
	if q, c := a.saldo(1); q != 12000 || c != 366667 {
		t.Errorf("saída: saldo %d, custo %d", q, c)
	}
	// o total do documento e do item vêm do cálculo sem arredondar, gravado uma vez em centavos
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "2:0.125:9.99"))
	var total, item int64
	a.banco.R.QueryRow(`SELECT docto_valor FROM Documento ORDER BY docto_codigo DESC LIMIT 1`).Scan(&total)
	a.banco.R.QueryRow(`SELECT itemdocto_total FROM Itens_Documento ORDER BY itemdocto_codigo DESC LIMIT 1`).Scan(&item)
	if total != 125 || item != 125 { // 0,125 × 9,99 = 1,24875 → 1,25
		t.Errorf("total %d, item %d (esperado 125 e 125)", total, item)
	}
}

func TestSaidaComSaldoInsuficienteENaoGravaNada(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:3:10"))
	n.lancar(docForm("SAIDA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:10:10"))
	n.esperaMensagem("/documentos/novo", "Estoque insuficiente para &#39;Produto A&#39;. Disponível: 3.000 | Solicitado: 10.000")
	// dois itens do mesmo produto que juntos passam do saldo: cada um passa sozinho, o segundo falha ao gravar e tudo é desfeito
	n.lancar(docForm("SAIDA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:2:10", "1:2:10"))
	n.esperaMensagem("/documentos/novo", "Estoque insuficiente para o produto &#39;Produto A&#39;. Disponível: 1.000 | Solicitado: 2.000")
	if q, _ := a.saldo(1); q != 3000 {
		t.Errorf("saldo = %d, esperado 3000 (nada pode ter mudado)", q)
	}
	if a.conta(`SELECT count(*) FROM Documento`) != 1 || a.conta(`SELECT count(*) FROM Itens_Documento`) != 1 {
		t.Error("o documento que falhou não pode ficar gravado")
	}
	// os códigos do documento e dos itens que falharam ficam queimados (como o IDENTITY do SQL Server)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:1:10"))
	if a.conta(`SELECT count(*) FROM Documento WHERE docto_codigo = 3`) != 1 || a.conta(`SELECT count(*) FROM Itens_Documento WHERE itemdocto_codigo = 4`) != 1 {
		t.Error("o próximo documento deveria ser o 3 e o próximo item o 4")
	}
}

func TestVendaGeraTitulosEParcelasComORestoNaUltima(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:10:5"))
	// Crédito 3x (forma 4, 12 parcelas, sem baixa automática): 100,00 → 33,33 + 33,33 + 33,34
	if loc := n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "4", "parcelas": "3", "primeiroVencimento": "2026-12-01"}, "1:10:10")); loc != "/documentos" {
		t.Fatalf("venda = %q", loc)
	}
	rows, err := a.banco.R.Query(`SELECT tit_parcela, tit_valor, tit_vencimento, tit_status, tit_natureza, tit_origem, tit_competencia, tit_descricao FROM Titulo ORDER BY tit_parcela`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var valores []int64
	var venc []string
	for rows.Next() {
		var p, v int64
		var vc, st, nat, orig, comp, desc string
		rows.Scan(&p, &v, &vc, &st, &nat, &orig, &comp, &desc)
		valores, venc = append(valores, v), append(venc, vc)
		if st != "ABERTO" || nat != "RECEBER" || orig != "DOCUMENTO" || comp != "2026-11-01" || desc != "Venda #2 — Cliente" {
			t.Errorf("título %d: %s %s %s %s %q", p, st, nat, orig, comp, desc)
		}
	}
	if len(valores) != 3 || valores[0] != 3333 || valores[1] != 3333 || valores[2] != 3334 {
		t.Errorf("parcelas = %v, esperado [3333 3333 3334]", valores)
	}
	if venc[0] != "2026-12-01" || venc[1] != "2026-12-31" || venc[2] != "2027-01-30" { // base + i × 30 dias
		t.Errorf("vencimentos = %v", venc)
	}
	if q, _ := a.saldo(1); q != 0 {
		t.Errorf("a venda deveria baixar o estoque: %d", q)
	}
}

func TestBaixaAutomaticaLancaMovimentoETaxaDeCartao(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:20:5"))
	// PIX (forma 2) passa a ter taxa de 2,50 % e baixa automática no Banco
	a.banco.W.Exec(`UPDATE Forma_Pagamento SET fpg_taxa_percentual = 250 WHERE fpg_codigo = 2`)
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "2"}, "1:10:10"))
	if a.conta(`SELECT count(*) FROM Titulo WHERE tit_origem = 'DOCUMENTO' AND tit_status = 'BAIXADO' AND tit_valor = 10000 AND tit_valor_baixado = 10000`) != 1 {
		t.Error("o título da venda deveria nascer baixado")
	}
	var taxa, bxTaxa int64
	var desc string
	if err := a.banco.R.QueryRow(`SELECT tit_valor, tit_descricao FROM Titulo WHERE tit_origem = 'TAXA' AND tit_natureza = 'PAGAR' AND tit_status = 'BAIXADO'`).Scan(&taxa, &desc); err != nil {
		t.Fatalf("faltou o título da taxa: %v", err)
	}
	if taxa != 250 || desc != "Taxa PIX — título #1" { // 100,00 × 2,50 % = 2,50
		t.Errorf("taxa %d, descrição %q", taxa, desc)
	}
	a.banco.R.QueryRow(`SELECT bx_taxa FROM Baixa WHERE bx_automatica = 1 AND bx_valor = 10000`).Scan(&bxTaxa)
	if bxTaxa != 250 {
		t.Errorf("bx_taxa = %d", bxTaxa)
	}
	var entrada, saida int64
	a.banco.R.QueryRow(`SELECT COALESCE(SUM(mov_valor),0) FROM Movimento_Conta WHERE mov_tipo = 'ENTRADA'`).Scan(&entrada)
	a.banco.R.QueryRow(`SELECT COALESCE(SUM(mov_valor),0) FROM Movimento_Conta WHERE mov_tipo = 'SAIDA'`).Scan(&saida)
	if entrada != 10000 || saida != 250 {
		t.Errorf("extrato: entrou %d, saiu %d (esperado 10000 e 250)", entrada, saida)
	}
	// excluir a venda apaga títulos, taxa, baixas e movimentos, e devolve o estoque
	n.postar("/documentos/excluir/2", nil)
	n.esperaMensagem("/documentos", "Documento excluído e estoque revertido!")
	for _, tab := range []string{"Titulo", "Baixa", "Movimento_Conta"} {
		if a.conta(`SELECT count(*) FROM `+tab) != 0 {
			t.Errorf("%s deveria ter ficado vazia", tab)
		}
	}
	if q, _ := a.saldo(1); q != 20000 {
		t.Errorf("saldo depois de excluir = %d", q)
	}
}

func TestBaixaManualEConciliacaoBloqueiamEditarEExcluir(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:20:5"))
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "5", "parcelas": "1"}, "1:2:10")) // boleto: título em aberto
	a.banco.W.Exec(`INSERT INTO Baixa (bx_tit_codigo, bx_data, bx_cfn_codigo, bx_valor, bx_automatica) VALUES (1,'2026-11-16',2,2000,0)`)
	n.postar("/documentos/excluir/2", nil)
	n.esperaMensagem("/documentos", "Erro ao excluir: O documento tem baixa lançada manualmente no título #1. Estorne a baixa no Financeiro antes de alterar ou excluir o documento.")
	if !strings.Contains(n.paginaOK("/documentos/editar/2"), "Este documento tem baixa lançada manualmente no financeiro. Estorne a baixa antes de alterá-lo.") {
		t.Error("a edição deveria avisar da baixa manual")
	}
	v := docForm("VENDA", map[string]string{"doctoCodigo": "2", "fpgCodigo": "5"}, "1:1:10")
	n.lancar(v)
	n.esperaMensagem("/documentos/editar/2", "O documento tem baixa lançada manualmente no título #1.")
	// baixa AUTOMÁTICA com movimento conciliado também bloqueia
	a.banco.W.Exec(`DELETE FROM Baixa`)
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1"}, "1:1:10"))
	a.banco.W.Exec(`UPDATE Movimento_Conta SET mov_conciliado = 1`)
	n.postar("/documentos/excluir/3", nil)
	n.esperaMensagem("/documentos", "Erro ao excluir: O lançamento da baixa #")
	if a.conta(`SELECT count(*) FROM Documento WHERE docto_codigo = 3`) != 1 {
		t.Error("o documento com baixa conciliada continua")
	}
}

func TestEditarEntradaJaConsumidaEExcluirComFalta(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:5:10"))
	n.lancar(docForm("SAIDA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:4:10"))
	n.postar("/documentos/excluir/1", nil)
	n.esperaMensagem("/documentos", "Erro ao excluir: Não é possível excluir: o estoque de &#39;Produto A&#39; já foi consumido (saldo atual: 1.000, precisaria reverter: 5.000).")
	n.lancar(docForm("ENTRADA", map[string]string{"doctoCodigo": "1", "doctoClassificacao": "AJUSTE"}, "1:5:10"))
	n.esperaMensagem("/documentos/editar/1", "Não é possível editar: o estoque de &#39;Produto A&#39; já foi consumido (saldo: 1.000, precisaria reverter: 5.000).")
	if q, _ := a.saldo(1); q != 1000 {
		t.Errorf("saldo = %d (a recusa não pode mexer)", q)
	}
	// excluir a saída devolve ao estoque, sem mexer no custo
	n.postar("/documentos/excluir/2", nil)
	if q, c := a.saldo(1); q != 5000 || c != 100000 {
		t.Errorf("depois de excluir a saída: saldo %d, custo %d", q, c)
	}
}

func TestEditarRefazEstoqueETitulos(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:20:5"))
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1"}, "1:5:10")) // Dinheiro: baixa automática
	if q, _ := a.saldo(1); q != 15000 {
		t.Fatalf("saldo %d", q)
	}
	// edita: 8 un., agora em boleto 2x
	n.lancar(docForm("VENDA", map[string]string{"doctoCodigo": "2", "fpgCodigo": "5", "parcelas": "2"}, "1:8:10"))
	n.esperaMensagem("/documentos", "Documento atualizado com sucesso!")
	if q, _ := a.saldo(1); q != 12000 {
		t.Errorf("saldo = %d, esperado 12000 (reverteu 5 e baixou 8)", q)
	}
	if a.conta(`SELECT count(*) FROM Titulo`) != 2 || a.conta(`SELECT count(*) FROM Baixa`) != 0 || a.conta(`SELECT count(*) FROM Movimento_Conta`) != 0 {
		t.Error("os títulos e a baixa automática antigos deveriam ter sumido, restando 2 títulos em aberto")
	}
	if a.conta(`SELECT count(*) FROM Itens_Documento WHERE itemdocto_docto_codigo = 2`) != 1 || a.conta(`SELECT count(*) FROM Documento WHERE docto_codigo = 2 AND docto_valor = 8000`) != 1 {
		t.Error("o documento e os itens deveriam refletir a edição")
	}
	// o usuário que lançou continua sendo o original
	var usu int64
	a.banco.R.QueryRow(`SELECT docto_usu_codigo FROM Documento WHERE docto_codigo = 2`).Scan(&usu)
	if usu != 1 {
		t.Errorf("usuário do documento = %d", usu)
	}
}

func TestVendaComFracaoDeCentavoEmBaixaAutomaticaD21(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:5:5"))
	// 0,5 × 9,99 = 4,995: o antigo recusava ("O valor da baixa (R$ 5.00) é maior que o saldo do título (R$ 4.995)"); o novo grava com o título arredondado
	if loc := n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "2"}, "1:0.5:9.99")); loc != "/documentos" {
		n.esperaMensagem(loc, "")
		t.Fatalf("a venda deveria ser gravada, loc = %q", loc)
	}
	if a.conta(`SELECT count(*) FROM Titulo WHERE tit_valor = 500 AND tit_valor_baixado = 500 AND tit_status = 'BAIXADO'`) != 1 {
		t.Error("o título deveria ter 5,00 e estar baixado")
	}
	if a.conta(`SELECT count(*) FROM Documento WHERE docto_valor = 500`) != 1 {
		t.Error("o documento deveria ter 5,00")
	}
}

func TestFormaDeNaturezaErradaInativaOuSemContaNaoGravaNada(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:5:5"))
	casos := []struct {
		campos map[string]string
		msg    string
	}{
		{map[string]string{"fpgCodigo": "8"}, "A forma &#39;PIX / transferência&#39; não é de recebimento."},
		{map[string]string{"fpgCodigo": "5", "parcelas": "7"}, "A forma &#39;Boleto&#39; permite de 1 a 6 parcelas."},
		{map[string]string{"fpgCodigo": "99"}, "Forma de pagamento não encontrada."},
		{map[string]string{"fpgCodigo": "5", "catCodigo": "99"}, "Categoria não encontrada."},
		{map[string]string{}, "Selecione a forma de pagamento do documento."},
	}
	for _, c := range casos {
		n.lancar(docForm("VENDA", c.campos, "1:1:10"))
		n.esperaMensagem("/documentos/novo", c.msg)
	}
	a.banco.W.Exec(`UPDATE Forma_Pagamento SET fpg_ativo = 0 WHERE fpg_codigo = 5`)
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "5"}, "1:1:10"))
	n.esperaMensagem("/documentos/novo", "A forma de pagamento &#39;Boleto&#39; está inativa.")
	a.banco.W.Exec(`UPDATE Forma_Pagamento SET fpg_ativo = 1 WHERE fpg_codigo = 5`)
	a.banco.W.Exec(`UPDATE Forma_Pagamento SET fpg_cfn_codigo = NULL WHERE fpg_codigo = 1`)
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1"}, "1:1:10"))
	n.esperaMensagem("/documentos/novo", "A forma &#39;Dinheiro&#39; baixa automaticamente, mas não tem conta de destino.")
	if a.conta(`SELECT count(*) FROM Documento`) != 1 || a.conta(`SELECT count(*) FROM Titulo`) != 0 {
		t.Error("nenhuma venda recusada pode ter ficado gravada")
	}
	if q, _ := a.saldo(1); q != 5000 {
		t.Errorf("saldo = %d", q)
	}
}

func TestSegmentoCategoriaEComissaoDoDocumento(t *testing.T) {
	a, n := montarLoja(t)
	n.postar("/produtos/salvar", produtoForm("Corte", "uprServico", "true", "uprPreco", "80", "uprComissaoPerc", "40", "segmentoId", "2", "uprCatVenda", "2"))
	n.drenar()
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:5:5"))
	a.banco.W.Exec(`UPDATE Produto SET upr_seg_codigo = 1 WHERE upr_codigo = 1`)
	// serviço com comissão exige o profissional
	v := docForm("VENDA", map[string]string{"fpgCodigo": "3"}, "3:1:80")
	n.lancar(v)
	n.esperaMensagem("/documentos/novo", "Informe o profissional do serviço &#39;Corte&#39; (ele tem comissão de 40%).")
	v = docForm("VENDA", map[string]string{"fpgCodigo": "3", "vendedorId": "3"}, "1:1:10", "3:2:80")
	v.Set("profissionalIds", "")
	v["profissionalIds"] = []string{"", "3"}
	n.lancar(v)
	var seg, cat int64
	a.banco.R.QueryRow(`SELECT docto_seg_codigo FROM Documento WHERE docto_codigo = 2`).Scan(&seg)
	if seg != 2 { // o serviço (2 × 80) vale mais que o produto (1 × 10): segmento Salão
		t.Errorf("segmento = %d, esperado 2 (do item de maior valor)", seg)
	}
	a.banco.R.QueryRow(`SELECT tit_cat_codigo FROM Titulo WHERE tit_docto_codigo = 2`).Scan(&cat)
	if cat != 2 { // a categoria de venda do serviço (maior soma): Outras receitas
		t.Errorf("categoria = %d, esperado 2", cat)
	}
	var perc, prof int64
	a.banco.R.QueryRow(`SELECT itemdocto_comissao_perc, itemdocto_prof_codigo FROM Itens_Documento WHERE itemdocto_docto_codigo = 2 AND itemdocto_upr_codigo = 3`).Scan(&perc, &prof)
	if perc != 4000 || prof != 3 {
		t.Errorf("comissão %d, profissional %d", perc, prof)
	}
	corpo := n.paginaOK("/documentos/visualizar/2")
	if !strings.Contains(corpo, "· comissão 40,00% (R$ 64,00)") {
		t.Errorf("a visualização deveria mostrar a comissão:\n%s", trecho(corpo, "comissão"))
	}
	// o item que não é serviço nunca guarda profissional nem comissão
	if a.conta(`SELECT count(*) FROM Itens_Documento WHERE itemdocto_docto_codigo = 2 AND itemdocto_upr_codigo = 1 AND itemdocto_prof_codigo IS NULL AND itemdocto_comissao_perc IS NULL`) != 1 {
		t.Error("produto comum não guarda profissional nem comissão")
	}
}

func TestEscopoDeVendasDoVendedor(t *testing.T) {
	a, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:20:5"))
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1", "vendedorId": "3"}, "1:1:10")) // doc 2, do vendedor 3
	n.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1"}, "1:1:10"))                    // doc 3, sem vendedor
	// "ana" vende (vendedor vinculado = pessoa 3) mas não vê as vendas de todos; "sem" não tem vendedor vinculado
	a.criarUsuario("ana", "Ana", "Ana-Senha-123", true, "DOCUMENTOS:CIAE", "PRODUTOS:C", "PESSOAS:C")
	a.criarUsuario("sem", "Sem", "Sem-Senha-123", true, "DOCUMENTOS:CIAE", "PRODUTOS:C", "PESSOAS:C")
	a.banco.W.Exec(`UPDATE Usuarios SET usu_ups_vendedor = 3 WHERE usu_login = 'ana'`)
	ana := a.novoNavegador()
	ana.entrar("ana", "Ana-Senha-123")
	lista := ana.paginaOK("/documentos")
	if !strings.Contains(lista, "/documentos/visualizar/2") || strings.Contains(lista, "/documentos/visualizar/3") || strings.Contains(lista, "/documentos/visualizar/1") {
		t.Errorf("ana deveria ver só o documento 2")
	}
	if resp, _ := ana.get("/documentos/visualizar/3"); resp.Header.Get("Location") != "/documentos" {
		t.Error("o documento de outro vendedor não abre")
	}
	ana.esperaMensagem("/documentos", "Documento não encontrado.")
	form := ana.paginaOK("/documentos/novo")
	if !strings.Contains(form, `disabled="disabled" title="Seu usuário só lança documentos no seu próprio nome."`) {
		t.Error("o vendedor do formulário deveria ficar travado")
	}
	// lançar sai sempre no nome do vendedor do usuário, mesmo que o formulário diga outro
	ana.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1", "vendedorId": "1"}, "1:1:10"))
	var vend int64
	a.banco.R.QueryRow(`SELECT docto_vendedor FROM Documento ORDER BY docto_codigo DESC LIMIT 1`).Scan(&vend)
	if vend != 3 {
		t.Errorf("vendedor do documento = %d, esperado 3", vend)
	}
	// alterar ou excluir documento de outro vendedor
	ana.lancar(docForm("VENDA", map[string]string{"doctoCodigo": "3", "fpgCodigo": "1"}, "1:1:10"))
	// a mensagem fica para a tela de edição, que a própria Ana não consegue abrir: ela cai na lista com "não encontrado" (como no antigo)
	ana.get("/documentos/editar/3")
	ana.esperaMensagem("/documentos", "Documento não encontrado.")
	ana.postar("/documentos/excluir/3", nil)
	ana.esperaMensagem("/documentos", "Erro ao excluir: Você só pode ver e alterar os documentos em que é o vendedor.")
	// sem vendedor vinculado: não vê nada e não lança
	sem := a.novoNavegador()
	sem.entrar("sem", "Sem-Senha-123")
	if strings.Contains(sem.paginaOK("/documentos"), "/documentos/visualizar/") {
		t.Error("sem vendedor vinculado não vê nenhum documento")
	}
	sem.lancar(docForm("VENDA", map[string]string{"fpgCodigo": "1"}, "1:1:10"))
	sem.esperaMensagem("/documentos/novo", "Seu usuário não tem vendedor vinculado. Peça ao administrador para vincular um vendedor ao seu usuário.")
}

func TestListaDeDocumentosFiltrosEOrdem(t *testing.T) {
	_, n := montarLoja(t)
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE", "doctoData": "2026-11-10"}, "1:5:5"))
	n.lancar(docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE", "doctoData": "2026-11-10"}, "1:5:5"))
	n.lancar(docForm("SAIDA", map[string]string{"doctoClassificacao": "AJUSTE", "doctoData": "2026-11-12"}, "1:1:5"))
	corpo := n.paginaOK("/documentos")
	i1, i2, i3 := strings.Index(corpo, "visualizar/1\""), strings.Index(corpo, "visualizar/2\""), strings.Index(corpo, "visualizar/3\"")
	if !(i3 < i2 && i2 < i1) { // data mais recente primeiro; na mesma data, o código maior primeiro (como o índice lido de trás para frente no antigo)
		t.Errorf("ordem inesperada: 3 em %d, 2 em %d, 1 em %d", i3, i2, i1)
	}
	if corpo := n.paginaOK("/documentos?tipo=SAIDA"); strings.Contains(corpo, "visualizar/1\"") || !strings.Contains(corpo, "visualizar/3\"") {
		t.Error("filtro de tipo incorreto")
	}
	if corpo := n.paginaOK("/documentos?inicio=2026-11-11&fim=2026-11-30"); strings.Contains(corpo, "visualizar/1\"") || !strings.Contains(corpo, "visualizar/3\"") {
		t.Error("filtro de datas incorreto")
	}
	if resp, _ := n.get("/documentos?inicio=abc"); resp.StatusCode != 400 {
		t.Errorf("data inválida = %d, esperado 400", resp.StatusCode)
	}
}

func TestTelaDeEstoqueArredondaComoOJava(t *testing.T) {
	a, n := montarLoja(t)
	// custo 35,1250 → "35,12" (HALF_EVEN do DecimalFormat) e valor em estoque 3,5 × 35,1250 = 122,9375 → "122,94"
	a.banco.W.Exec(`UPDATE Estoque SET est_qtd = 3500, est_valor_custo = 351250 WHERE est_upr_codigo = 1`)
	a.banco.W.Exec(`UPDATE Estoque SET est_qtd = 0, est_valor_custo = 0 WHERE est_upr_codigo = 2`)
	corpo := n.paginaOK("/estoque")
	for _, s := range []string{"R$ 35,12", "R$ 122,94", "3,500", `class="row-warning"`, `class="row-danger"`, "Estoque baixo", "Sem estoque"} {
		if !strings.Contains(corpo, s) {
			t.Errorf("tela de estoque sem %q", s)
		}
	}
	// o valor total é sempre o do estoque INTEIRO, mesmo no filtro de estoque baixo
	baixo := n.paginaOK("/estoque?filtro=baixo")
	if !strings.Contains(baixo, "Valor Total:") || !strings.Contains(baixo, "R$ 122,94") {
		t.Error("filtro baixo")
	}
	a.banco.W.Exec(`UPDATE Estoque SET est_qtd = 6000 WHERE est_upr_codigo = 2`)
	if strings.Contains(n.paginaOK("/estoque?filtro=baixo"), "Produto B") {
		t.Error("6 un. não é estoque baixo (limite: até 5)")
	}
	if !strings.Contains(n.paginaOK("/estoque"), "Normal") {
		t.Error("deveria aparecer como Normal")
	}
}

func TestDocumentoParametrosRuinsESemPermissao(t *testing.T) {
	a, n := montarLoja(t)
	for _, c := range []struct{ nome, caminho string }{{"id", "/documentos/editar/abc"}, {"id", "/documentos/visualizar/abc"}} {
		if resp, _ := n.get(c.caminho); resp.StatusCode != 400 {
			t.Errorf("%s: %d, esperado 400", c.caminho, resp.StatusCode)
		}
	}
	v := url.Values{"doctoTipo": {"ENTRADA"}, "_csrf": {n.csrfLogado()}} // sem as listas obrigatórias
	if resp, _ := n.fazer("POST", "/documentos/salvar", v, nil); resp.StatusCode != 400 {
		t.Errorf("sem produtoIds = %d, esperado 400", resp.StatusCode)
	}
	bad := docForm("ENTRADA", map[string]string{"doctoClassificacao": "AJUSTE"}, "1:abc:5")
	bad.Set("_csrf", n.csrfLogado())
	if resp, _ := n.fazer("POST", "/documentos/salvar", bad, nil); resp.StatusCode != 400 {
		t.Errorf("quantidade não numérica = %d, esperado 400", resp.StatusCode)
	}
	// permissões: só Consulta vê a lista, sem os botões; o formulário novo exige Inclusão
	a.criarUsuario("leitor", "Leitor", "Leitor-Senha-1", true, "DOCUMENTOS:C", "VENDAS_TODOS:C")
	l := a.novoNavegador()
	l.entrar("leitor", "Leitor-Senha-1")
	corpo := l.paginaOK("/documentos")
	for _, nao := range []string{"Novo Documento", "/documentos/editar/", "/documentos/excluir/"} {
		if strings.Contains(corpo, nao) {
			t.Errorf("quem só consulta não deveria ver %q", nao)
		}
	}
	l.get("/documentos/novo")
	l.esperaMensagem("/dashboard", "Você não tem permissão para incluir registros em Documentos.")
	l.postar("/documentos/excluir/1", nil)
	l.esperaMensagem("/dashboard", "Você não tem permissão para excluir registros de Documentos.")
}
