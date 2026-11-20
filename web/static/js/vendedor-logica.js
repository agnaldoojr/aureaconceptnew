/**
 * Aurea Concept — Modo vendedor: contas e regras (sem tela, sem rede).
 *
 * É a versão em JavaScript do que o aplicativo Android faz em Formatos.kt, Models.kt e AppViewModel.kt
 * (unidades e conversão, valor do item, bloqueio de estoque, parcelas, máscara e conferência de datas).
 * Fica separado da tela para ser testado sozinho: `node --test src/test/js` (no navegador vira window.VendedorLogica).
 */
(function (raiz, fabrica) {
  'use strict';
  var logica = fabrica();
  if (typeof module === 'object' && module.exports) module.exports = logica;
  else raiz.VendedorLogica = logica;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  function num(v) { return typeof v === 'number' ? v : Number(v); }

  /** Arredonda sem o erro de ponto flutuante (0,1 + 0,2): 1,005 com 2 casas dá 1,01. */
  function arredondar(valor, casas) {
    var f = Math.pow(10, casas);
    return Math.round((num(valor) + Number.EPSILON) * f) / f;
  }

  // ───────────────────────  formatação em português do Brasil  ───────────────────────

  /** R$ 1.234,50 */
  function moeda(valor) {
    return 'R$ ' + arredondar(valor, 2).toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }

  /** Quantidade sem zeros inúteis: 2 -> "2", 1.5 -> "1,5", 0.125 -> "0,125". */
  function quantidade(valor) {
    return arredondar(valor, 3).toLocaleString('pt-BR', { minimumFractionDigits: 0, maximumFractionDigits: 3 });
  }

  /** "2026-11-14" -> "14/11/2026". Qualquer outra coisa volta como veio. */
  function data(iso) {
    if (iso === null || iso === undefined) return '';
    var p = String(iso).slice(0, 10).split('-');
    return p.length === 3 && p[0].length === 4 ? p[2] + '/' + p[1] + '/' + p[0] : String(iso);
  }

  /** Máscara de data enquanto digita: só números, no máximo 8 ("14111990" -> "14/11/1990"). */
  function mascaraData(texto) {
    var d = String(texto).replace(/\D/g, '').slice(0, 8);
    if (d.length <= 2) return d;
    if (d.length <= 4) return d.slice(0, 2) + '/' + d.slice(2);
    return d.slice(0, 2) + '/' + d.slice(2, 4) + '/' + d.slice(4);
  }

  function iso(d) {
    return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
  }

  /**
   * "14/11/1990" -> "1990-11-14" (formato do servidor). Vazio = null (a data é opcional).
   * Lança Error com a mensagem para o usuário se a data não existir ou for futura.
   */
  function dataBrParaIso(texto, hoje) {
    var t = String(texto === null || texto === undefined ? '' : texto).trim();
    if (!t) return null;
    var invalida = new Error('Data de nascimento inválida. Use dia/mês/ano, por exemplo 14/11/1990.');
    var p = t.split('/');
    if (p.length !== 3 || p[2].length !== 4 || !/^\d+$/.test(p[0]) || !/^\d+$/.test(p[1]) || !/^\d+$/.test(p[2])) throw invalida;
    var dia = parseInt(p[0], 10), mes = parseInt(p[1], 10), ano = parseInt(p[2], 10);
    var d = new Date(ano, mes - 1, dia);
    if (d.getFullYear() !== ano || d.getMonth() !== mes - 1 || d.getDate() !== dia) throw invalida;   // ex.: 31/02
    var limite = iso(hoje || new Date());
    if (iso(d) > limite) throw new Error('A data de nascimento não pode ser futura.');
    return iso(d);
  }

  /** Aceita "1,5", "1.5" e "1.234,56"; devolve null se não for um número. */
  function lerDecimal(texto) {
    var t = String(texto === null || texto === undefined ? '' : texto).trim();
    if (!t) return null;
    var normal = t.indexOf(',') >= 0 ? t.replace(/\./g, '').replace(',', '.') : t;
    if (!/^[+-]?(\d+\.?\d*|\.\d+)$/.test(normal)) return null;
    var n = Number(normal);
    return isFinite(n) ? n : null;
  }

  /** Texto de um número para preencher um campo editável: 89.9 -> "89,90". */
  function paraCampo(valor, casas) {
    return arredondar(valor, casas).toFixed(casas).replace('.', ',');
  }

  // ───────────────────────  produto, unidades e itens  ───────────────────────

  function igual(a, b) { return String(a).toLowerCase() === String(b).toLowerCase(); }

  /** Todas as siglas em que dá para vender: a base primeiro. */
  function siglas(p) {
    return [p.unidade].concat((p.unidades || []).map(function (u) { return u.sigla; }));
  }

  function extra(p, sigla) {
    if (!sigla || igual(sigla, p.unidade)) return null;
    var lista = p.unidades || [];
    for (var i = 0; i < lista.length; i++) if (igual(lista[i].sigla, sigla)) return lista[i];
    return null;
  }

  /** Quantas unidades base cabem em 1 da unidade informada (base ou desconhecida = 1). */
  function fatorDe(p, sigla) { var e = extra(p, sigla); return e ? num(e.fator) : 1; }

  /** Preço de tabela na unidade informada. */
  function precoDe(p, sigla) { var e = extra(p, sigla); return e ? num(e.preco) : num(p.preco); }

  /** Quantidade do item em unidades base do produto (é nela que o estoque baixa). */
  function quantidadeBase(item) { return arredondar(num(item.quantidade) * fatorDe(item.produto, item.unidade), 3); }

  function totalItem(item) { return arredondar(num(item.quantidade) * num(item.valor), 2); }

  function valorTotal(itens) {
    return arredondar(itens.reduce(function (a, i) { return a + totalItem(i); }, 0), 2);
  }

  function quantidadeTotalBase(itens) {
    return arredondar(itens.reduce(function (a, i) { return a + quantidadeBase(i); }, 0), 3);
  }

  /** O pedido só pode ser enviado com cliente, forma de pagamento e pelo menos um item. */
  function pedidoPronto(r) { return !!(r && r.cliente && r.forma && r.itens && r.itens.length > 0); }

  /** A quantidade digitada (já convertida para a base) passa do saldo? */
  function passouDoEstoque(p, unidade, quantidadeTexto) {
    var q = lerDecimal(quantidadeTexto);
    if (p === null || p === undefined || q === null) return false;
    return arredondar(q * fatorDe(p, unidade), 3) > arredondar(p.estoque, 3);
  }

  /**
   * Confere o item da tela "Atualiza Item" (mesmas mensagens do aplicativo).
   * e = { produto, unidade, quantidade (texto), valor (texto), indice (null = item novo) }; itens = itens do pedido.
   * Devolve { item } se estiver certo ou { erro } com a mensagem.
   */
  function validarItem(e, itens) {
    var p = e.produto;
    if (!p) return { erro: 'Escolha o produto.' };
    var q = lerDecimal(e.quantidade);
    if (q === null || q <= 0) return { erro: 'Informe uma quantidade maior que zero.' };
    var v = lerDecimal(e.valor);
    if (v === null || v <= 0) return { erro: 'Informe o valor unitário.' };
    var sigla = e.unidade || p.unidade;
    // o saldo está na unidade base; soma o que já está em outros itens do mesmo produto
    var jaNoPedido = 0;
    itens.forEach(function (it, i) {
      if (it.produto.codigo === p.codigo && i !== e.indice) jaNoPedido += quantidadeBase(it);
    });
    var pedidoBase = arredondar(q * fatorDe(p, sigla), 3);
    if (arredondar(pedidoBase + jaNoPedido, 3) > arredondar(p.estoque, 3)) {
      return { erro: 'Estoque insuficiente: disponível ' + quantidade(p.estoque) + ' ' + p.unidade + '.' };
    }
    return { item: { produto: p, quantidade: q, valor: v, unidade: sigla } };
  }

  // ───────────────────────  pedido, forma de pagamento e cliente  ───────────────────────

  /** A forma "à vista" é a sugerida; sem ela, a primeira da lista. */
  function formaPadrao(formas) {
    for (var i = 0; i < formas.length; i++) if (formas[i].aVista) return formas[i];
    return formas.length ? formas[0] : null;
  }

  /** Parcelas dentro de 1 e o máximo da forma (forma não parcelável = 1). */
  function ajustarParcelas(forma, parcelas) {
    if (!forma || !forma.parcelavel) return 1;
    var max = Math.max(1, forma.maxParcelas || 1);
    return Math.min(max, Math.max(1, parseInt(parcelas, 10) || 1));
  }

  /** O que vai para o servidor (mesmo formato do aplicativo Android). */
  function montarPedido(r) {
    return {
      clienteId: r.cliente.codigo,
      formaPagamentoId: r.forma.codigo,
      parcelas: r.parcelas,
      itens: r.itens.map(function (i) {
        return { produtoId: i.produto.codigo, quantidade: i.quantidade, valor: i.valor, unidade: i.unidade };
      })
    };
  }

  /** Cadastro rápido de cliente: nome obrigatório, nascimento opcional. Devolve { cliente } ou { erro }. */
  function validarCliente(c, hoje) {
    var nome = String(c.nome || '').trim();
    if (!nome) return { erro: 'Informe o nome do cliente.' };
    var nascimento;
    try { nascimento = dataBrParaIso(c.nascimento, hoje); } catch (e) { return { erro: e.message }; }
    function opcional(t) { var s = String(t || '').trim(); return s ? s : null; }
    return {
      cliente: {
        nome: nome,
        documento: opcional(c.documento),
        telefone: opcional(c.telefone),
        nascimento: nascimento,
        segmentos: c.segmentos || []
      }
    };
  }

  /** Texto da última venda do produto (ou a explicação de que não há). */
  function textoUltimaVenda(p, u) {
    if (!p) return '—';
    if (!u) return 'Sem vendas anteriores deste produto.';
    return 'Últ. venda: ' + data(u.data) + '   Qtd: ' + quantidade(u.quantidade) + (u.unidade ? ' ' + u.unidade : '') +
      '   Vlr: ' + moeda(u.valor);
  }

  /** "#12 · Cartão (2x) · 3 itens" etc. para a linha do pedido. */
  function resumoPedido(p) {
    var partes = [data(p.data)];
    if (p.formaPagamento) partes.push(p.parcelas > 1 ? p.formaPagamento + ' (' + p.parcelas + 'x)' : p.formaPagamento);
    partes.push(p.quantidadeItens + (p.quantidadeItens === 1 ? ' item' : ' itens'));
    return partes.join(' · ');
  }

  /** Escapa texto para colocar dentro de HTML. */
  function esc(t) {
    return String(t === null || t === undefined ? '' : t)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  return {
    arredondar: arredondar, moeda: moeda, quantidade: quantidade, data: data, mascaraData: mascaraData,
    dataBrParaIso: dataBrParaIso, lerDecimal: lerDecimal, paraCampo: paraCampo,
    siglas: siglas, fatorDe: fatorDe, precoDe: precoDe, quantidadeBase: quantidadeBase, totalItem: totalItem,
    valorTotal: valorTotal, quantidadeTotalBase: quantidadeTotalBase, pedidoPronto: pedidoPronto,
    passouDoEstoque: passouDoEstoque, validarItem: validarItem, formaPadrao: formaPadrao,
    ajustarParcelas: ajustarParcelas, montarPedido: montarPedido, validarCliente: validarCliente,
    textoUltimaVenda: textoUltimaVenda, resumoPedido: resumoPedido, esc: esc
  };
});
