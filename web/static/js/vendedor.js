/**
 * Aurea Concept — Modo vendedor (telas de celular).
 *
 * Reproduz o fluxo do aplicativo Android: abas Pedidos e Estoque, novo pedido, escolher cliente e produto,
 * cadastro rápido de cliente, item com unidade e última venda, detalhe do pedido. Os dados vêm de /vendedor/api,
 * que usa o mesmo código da API do aplicativo (regras de estoque, parcelas e títulos ficam no servidor).
 * As contas ficam em vendedor-logica.js (testado em src/test/js).
 */
(function () {
  'use strict';

  var L = window.VendedorLogica;
  var app = document.getElementById('vd-app');
  var avisoEl = document.getElementById('vd-aviso');
  var esc = L.esc;

  function meta(nome) { var m = document.querySelector('meta[name="' + nome + '"]'); return m ? m.getAttribute('content') : ''; }
  var CSRF = { token: meta('csrf-token'), header: meta('csrf-header') };

  var estado = {
    usuario: null,
    aba: 'pedidos',
    filtroEstoque: 'todos',
    buscandoHome: false,
    buscaHome: '',
    pedidos: [], produtos: [], produtosPedido: [], clientes: [], formas: [], segmentos: [],
    carregando: 0,
    rascunho: { cliente: null, forma: null, parcelas: 1, itens: [] },
    edicao: null,
    segmentosMarcados: [],
    menuAberto: false,
    formaAberta: false,
    unidadeAberta: false,
    forcarAba: null,
    pedidoAberto: null
  };

  // ───────────────────────  rede  ───────────────────────

  function erro(status, mensagem) { var e = new Error(mensagem); e.status = status; return e; }

  function api(caminho, corpo) {
    var init = { method: corpo === undefined ? 'GET' : 'POST', credentials: 'same-origin', headers: { 'Accept': 'application/json' } };
    if (corpo !== undefined) {
      init.headers['Content-Type'] = 'application/json';
      init.headers[CSRF.header] = CSRF.token;
      init.body = JSON.stringify(corpo);
    }
    return fetch('/vendedor/api' + caminho, init).then(function (res) {
      // sessão expirada: o site responde com a página de login
      if (res.redirected && /\/login/.test(res.url)) { location.href = '/login?expired=true'; throw erro(401, 'Sessão expirada.'); }
      var tipo = res.headers.get('content-type') || '';
      if (tipo.indexOf('json') < 0) {
        if (res.status === 403) throw erro(403, 'Sua sessão não é mais válida. Atualize a página e entre de novo.');
        throw erro(res.status, 'Resposta inesperada do servidor.');
      }
      return res.json().catch(function () { return null; }).then(function (dados) {
        if (!res.ok) throw erro(res.status, (dados && dados.erro) || ('Erro ' + res.status));
        return dados;
      });
    }, function () {
      throw erro(0, 'Não foi possível conectar ao servidor. Confira a internet.');
    });
  }

  function consulta(caminho, params) {
    var q = [];
    Object.keys(params || {}).forEach(function (k) {
      if (params[k] !== '' && params[k] !== null && params[k] !== undefined) q.push(k + '=' + encodeURIComponent(params[k]));
    });
    return api(caminho + (q.length ? '?' + q.join('&') : ''));
  }

  function aviso(texto, ehErro) {
    avisoEl.textContent = texto;
    avisoEl.className = 'vd-aviso' + (ehErro ? ' erro' : '');
    avisoEl.removeAttribute('hidden');
    clearTimeout(aviso.t);
    aviso.t = setTimeout(function () { avisoEl.setAttribute('hidden', 'hidden'); }, 4500);
  }

  function barra() {
    var b = document.getElementById('vd-barra');
    if (b) b.className = 'vd-barra-progresso' + (estado.carregando > 0 ? ' on' : '');
  }

  /** Executa uma chamada mostrando a barra de progresso; erros viram aviso (401 volta ao login). */
  function executar(fn) {
    estado.carregando++; barra();
    return Promise.resolve().then(fn).catch(function (e) {
      if (e && e.status === 401) { location.href = '/login?expired=true'; return; }
      aviso((e && e.message) || 'Algo deu errado.', true);
    }).then(function () { estado.carregando--; barra(); atualizarBotaoEnviar(); });
  }

  function atrasar(fn, ms) {
    var t;
    return function () { var a = arguments; clearTimeout(t); t = setTimeout(function () { fn.apply(null, a); }, ms); };
  }

  // ───────────────────────  rascunho do pedido (guardado no aparelho)  ───────────────────────

  function chaveRascunho() { return 'aurea-vd-rascunho-' + (estado.usuario ? estado.usuario.codigo : '0'); }

  function guardarRascunho() {
    try { window.localStorage.setItem(chaveRascunho(), JSON.stringify(estado.rascunho)); } catch (e) { /* sem armazenamento: vale só nesta tela */ }
  }

  function lerRascunho() {
    try {
      var r = JSON.parse(window.localStorage.getItem(chaveRascunho()) || 'null');
      if (r && Array.isArray(r.itens)) {
        return { cliente: r.cliente || null, forma: r.forma || null, parcelas: r.parcelas || 1, itens: r.itens.filter(function (i) { return i && i.produto; }) };
      }
    } catch (e) { /* rascunho corrompido: começa vazio */ }
    return { cliente: null, forma: null, parcelas: 1, itens: [] };
  }

  function rascunhoTemConteudo() { var r = estado.rascunho; return !!(r.cliente || r.itens.length); }

  function limparRascunho() {
    estado.rascunho = { cliente: null, forma: null, parcelas: 1, itens: [] };
    guardarRascunho();
  }

  // ───────────────────────  navegação (pilha de telas pelo histórico do navegador)  ───────────────────────

  function rotaAtual() { var h = (location.hash || '').replace(/^#/, ''); return h || '/'; }
  function profundidade() { return (history.state && history.state.vd) || 0; }

  function ir(rota) {
    history.pushState({ vd: profundidade() + 1 }, '', '#' + rota);
    desenhar();
  }

  /** Volta n telas; se a pilha não tiver tantas (tela aberta direto), vai para o destino indicado. */
  function voltar(n, destino) {
    n = n || 1;
    if (profundidade() >= n) { history.go(-n); return; }
    history.replaceState({ vd: 0 }, '', '#' + (destino || '/'));
    desenhar();
  }

  /** Volta ao início (Pedidos), como o aplicativo faz depois de enviar um pedido. */
  function irParaInicio() {
    estado.forcarAba = 'pedidos';
    var d = profundidade();
    if (d > 0) history.go(-d); else { history.replaceState({ vd: 0 }, '', '#/'); desenhar(); }
  }

  window.addEventListener('popstate', desenhar);

  // ───────────────────────  carregamento de listas  ───────────────────────

  function carregarPedidos() {
    return executar(function () {
      return consulta('/pedidos', { busca: estado.buscaHome }).then(function (l) { estado.pedidos = l; atualizarLista(); });
    });
  }

  function carregarEstoque() {
    return executar(function () {
      return consulta('/produtos', { busca: estado.buscaHome, estoque: estado.filtroEstoque }).then(function (l) { estado.produtos = l; atualizarLista(); });
    });
  }

  function carregarClientes(busca) {
    return executar(function () {
      return consulta('/clientes', { busca: busca }).then(function (l) { estado.clientes = l; atualizarLista(); });
    });
  }

  function carregarProdutosParaPedido(busca) {
    return executar(function () {
      return consulta('/produtos', { busca: busca, estoque: 'com' }).then(function (l) { estado.produtosPedido = l; atualizarLista(); });
    });
  }

  function garantirFormas() {
    if (estado.formas.length) return Promise.resolve();
    return consulta('/formas-pagamento').then(function (f) { estado.formas = f; });
  }

  function buscarUltimaVenda(e) {
    if (!e.produto) return;
    var cliente = estado.rascunho.cliente ? estado.rascunho.cliente.codigo : '';
    var codigo = e.produto.codigo;
    consulta('/produtos/' + codigo + '/ultima-venda', { clienteId: cliente }).then(function (u) {
      if (estado.edicao === e && e.produto && e.produto.codigo === codigo) { e.ultimaVenda = u; pintarUltimaVenda(); }
    }, function () {
      if (estado.edicao === e) { e.ultimaVenda = null; pintarUltimaVenda(); }
    });
  }

  // ───────────────────────  peças de tela  ───────────────────────

  function topo(titulo, opcoes) {
    opcoes = opcoes || {};
    var h = '<header class="vd-topo">';
    if (opcoes.voltar) h += '<button class="vd-icone" data-acao="voltar" aria-label="Voltar"><i class="fas fa-arrow-left"></i></button>';
    h += opcoes.miolo || '<h1>' + esc(titulo) + '</h1>';
    h += opcoes.acoes || '';
    h += '</header><div id="vd-barra" class="vd-barra-progresso' + (estado.carregando > 0 ? ' on' : '') + '"></div>';
    return h;
  }

  function topoComBusca(valor, onde) {
    return topo('', {
      voltar: true,
      miolo: '<input id="vd-busca" class="vd-topo-busca" type="search" inputmode="search" autocomplete="off" placeholder="Pesquisa" value="' + esc(valor) + '" data-onde="' + onde + '">'
    });
  }

  function linhaProduto(p, i, clicavel) {
    var tambem = (p.unidades && p.unidades.length)
      ? ' · também: ' + p.unidades.map(function (u) { return esc(u.sigla) + ' (' + L.quantidade(u.fator) + ' ' + esc(p.unidade) + ')'; }).join(', ') : '';
    return '<div class="vd-linha' + (clicavel ? '' : ' sem-toque') + '"' + (clicavel ? ' data-acao="produto-pick" data-i="' + i + '"' : '') + '>' +
      '<div class="vd-linha-l1"><div style="min-width:0"><strong>' + p.codigo + ' - ' + esc(p.nome) + '</strong>' +
      '<div class="vd-sub">' + esc(p.unidade) + ' · ' + L.moeda(p.preco) + tambem + '</div></div>' +
      '<div class="vd-estoque"><b class="' + (L.arredondar(p.estoque, 3) <= 0 ? 'vd-ruim' : 'vd-ok') + '">' + L.quantidade(p.estoque) + '</b><span>em estoque</span></div></div></div>';
  }

  function vazio(texto) { return '<div class="vd-vazio">' + esc(texto || 'Nenhum registro encontrado') + '</div>'; }

  // ───────────────────────  telas  ───────────────────────

  function telaHome() {
    var u = estado.usuario;
    var pode = u && u.podeLancar;
    var acoes = '<button class="vd-icone" data-acao="busca-toggle" aria-label="Pesquisar"><i class="fas fa-magnifying-glass"></i></button>';
    if (estado.aba === 'pedidos' && pode) acoes += '<button class="vd-icone" data-acao="novo-pedido" aria-label="Novo pedido"><i class="fas fa-plus"></i></button>';
    acoes += '<button class="vd-icone" data-acao="menu" aria-label="Mais"><i class="fas fa-ellipsis-vertical"></i></button>';

    var h = topo(estado.aba === 'pedidos' ? 'Pedidos' : 'Estoque', { acoes: acoes });
    h += '<main class="vd-corpo">';
    if (estado.buscandoHome) {
      h += '<label class="vd-campo" style="margin:.5rem 1rem 0"><input id="vd-busca" type="search" inputmode="search" autocomplete="off" placeholder="Pesquisa" value="' + esc(estado.buscaHome) + '" data-onde="home"></label>';
    }
    if (estado.aba === 'estoque') {
      h += '<div class="vd-chips">' + [['todos', 'Todos'], ['com', 'Com estoque'], ['sem', 'Sem estoque']].map(function (f) {
        return '<button class="vd-chip' + (estado.filtroEstoque === f[0] ? ' on' : '') + '" data-acao="filtro" data-f="' + f[0] + '">' + f[1] + '</button>';
      }).join('') + '</div>';
    }
    if (estado.aba === 'pedidos' && u && !pode) {
      h += '<div class="vd-alerta">' + (u.vendedor ? 'Seu usuário não tem permissão para lançar pedidos.'
        : 'Seu usuário não tem vendedor vinculado: você pode consultar o estoque, mas não lançar pedidos. Peça ao administrador para vincular um vendedor.') + '</div>';
    }
    h += '<div id="vd-lista"></div></main>';
    h += '<nav class="vd-abas">' +
      '<button class="vd-aba' + (estado.aba === 'pedidos' ? ' on' : '') + '" data-acao="aba" data-aba="pedidos"><i class="fas fa-house"></i>Pedidos</button>' +
      '<button class="vd-aba' + (estado.aba === 'estoque' ? ' on' : '') + '" data-acao="aba" data-aba="estoque"><i class="fas fa-list"></i>Estoque</button></nav>';
    if (estado.menuAberto) h += menuUsuario();
    return h;
  }

  function menuUsuario() {
    var u = estado.usuario;
    return '<div class="vd-fundo menu" data-acao="menu-fechar"><div class="vd-menu" data-acao="nada">' +
      '<div class="vd-opcao so-texto">' + esc(u && u.vendedor ? 'Vendedor: ' + u.vendedor.nome : (u ? u.nome : '')) + '</div>' +
      '<div class="vd-opcao" data-acao="atualizar">Atualizar</div>' +
      '<a class="vd-opcao" href="/dashboard" style="display:block;color:inherit">Painel completo</a>' +
      '<a class="vd-opcao" href="/logout" style="display:block;color:inherit">Sair</a></div></div>';
  }

  function listaHome() {
    if (estado.aba === 'pedidos') {
      if (!estado.pedidos.length) return estado.carregando ? '' : vazio();
      return estado.pedidos.map(function (p) {
        return '<div class="vd-linha" data-acao="pedido-abrir" data-id="' + p.codigo + '"><div class="vd-linha-l1">' +
          '<strong>#' + p.codigo + ' · ' + esc(p.cliente) + '</strong><span class="vd-valor">' + L.moeda(p.valor) + '</span></div>' +
          '<div class="vd-sub">' + esc(L.resumoPedido(p)) + '</div></div>';
      }).join('');
    }
    if (!estado.produtos.length) return estado.carregando ? '' : vazio();
    return estado.produtos.map(function (p, i) { return linhaProduto(p, i, false); }).join('');
  }

  function telaDetalhe() {
    var p = estado.pedidoAberto;
    if (!p) return null;
    var h = topo('Pedido #' + p.codigo, { voltar: true });
    h += '<main class="vd-corpo"><div class="vd-detalhe-topo"><h2>' + esc(p.cliente) + '</h2>' +
      '<div>Data: ' + L.data(p.data) + '</div>' +
      (p.vendedor ? '<div>Vendedor: ' + esc(p.vendedor) + '</div>' : '') +
      (p.formaPagamento ? '<div>Pagamento: ' + esc(p.formaPagamento) + (p.parcelas > 1 ? ' em ' + p.parcelas + 'x' : '') + '</div>' : '') + '</div>';
    h += (p.itens || []).map(function (i) {
      return '<div class="vd-linha sem-toque"><strong style="font-size:15px;font-weight:600">' + esc(i.produto) + '</strong>' +
        '<div class="vd-item-l2"><span>' + L.quantidade(i.quantidade) + ' ' + esc(i.unidade) + ' × ' + L.moeda(i.valor) + '</span>' +
        '<b style="font-weight:600;color:var(--dark)">' + L.moeda(i.total) + '</b></div></div>';
    }).join('');
    h += '</main><div class="vd-total-final"><span>Total</span><span class="vd-valor">' + L.moeda(p.valor) + '</span></div>';
    return h;
  }

  function telaNovoPedido() {
    var r = estado.rascunho;
    var d = new Date();
    var hoje = String(d.getDate()).padStart(2, '0') + '/' + String(d.getMonth() + 1).padStart(2, '0') + '/' + d.getFullYear();   // data de hoje no aparelho
    var h = topo('Novo pedido', {
      voltar: true,
      acoes: '<button id="vd-enviar" class="vd-icone" data-acao="enviar" aria-label="Enviar pedido"' + (L.pedidoPronto(r) && !estado.carregando ? '' : ' disabled') + '><i class="fas fa-paper-plane"></i></button>'
    });
    h += '<main class="vd-corpo">';
    h += '<div class="vd-cabeca"><div style="flex:1;min-width:0" data-acao="escolher-cliente"><div class="vd-rot">Pessoa</div>' +
      '<div class="vd-escolher' + (r.cliente ? '' : ' vazio') + '">' + (r.cliente ? r.cliente.codigo + ' - ' + esc(r.cliente.nome) : 'Toque para escolher o cliente') + '</div></div>' +
      '<div style="text-align:right"><div class="vd-rot" style="color:var(--silver-dark)">Movimento</div><div style="color:var(--silver-dark);font-weight:600">' + hoje + '</div></div></div>';
    h += '<div class="vd-fab-linha"><button class="vd-fab" data-acao="item-abrir" data-i="novo" aria-label="Adicionar item"><i class="fas fa-plus"></i></button></div>';
    h += '<div id="vd-itens">' + itensPedido() + '</div></main>';
    h += '<div id="vd-totais">' + totaisPedido() + '</div>';
    h += '<div class="vd-rodape" id="vd-rodape">' + rodapePedido() + '</div>';
    if (estado.formaAberta) h += folhaFormas();
    return h;
  }

  function itensPedido() {
    return estado.rascunho.itens.map(function (item, i) {
      var base = item.produto.unidade;
      var equivale = String(item.unidade).toLowerCase() === String(base).toLowerCase() ? '' : ' (= ' + L.quantidade(L.quantidadeBase(item)) + ' ' + esc(base) + ')';
      return '<div class="vd-item" data-acao="item-abrir" data-i="' + i + '"><div class="vd-item-corpo"><strong>' + item.produto.codigo + ' - ' + esc(item.produto.nome) + '</strong>' +
        '<div class="vd-item-l2"><span>Qtd ' + L.quantidade(item.quantidade) + ' ' + esc(item.unidade) + equivale + '</span>' +
        '<span>' + L.moeda(item.valor) + '  =  ' + L.moeda(L.totalItem(item)) + '</span></div></div>' +
        '<button class="vd-remover" data-acao="item-remover" data-i="' + i + '" aria-label="Remover item"><i class="fas fa-trash"></i></button></div>';
    }).join('');
  }

  function totaisPedido() {
    var it = estado.rascunho.itens;
    return '<div class="vd-totais"><div><div class="vd-rot">Qtd (base)</div><b>' + L.quantidade(L.quantidadeTotalBase(it)) + '</b></div>' +
      '<div><div class="vd-rot">VL Bruto</div><b>' + L.moeda(L.valorTotal(it)) + '</b></div>' +
      '<div><div class="vd-rot">VL Líquido</div><b class="forte">' + L.moeda(L.valorTotal(it)) + '</b></div></div>';
  }

  function rodapePedido() {
    var r = estado.rascunho, f = r.forma;
    var h = '<div class="vd-rot" style="color:var(--silver-dark)">Tipo de Documento</div><div style="font-size:16px">VENDA</div>' +
      '<div class="vd-rot" style="color:var(--silver-dark);margin-top:.6rem">Forma de Pagamento</div>' +
      '<div class="vd-clicavel" data-acao="forma-abrir">' + (f ? esc(f.nome) + ' — ' + esc(f.descricao) : 'Toque para escolher') + '</div>';
    if (f && f.parcelavel) {
      h += '<div class="vd-parcelas"><span class="vd-rot" style="color:var(--silver-dark)">Parcelas</span>' +
        '<button class="vd-passo" data-acao="parcelas" data-d="-1" aria-label="Menos uma parcela">−</button><b>' + r.parcelas + 'x</b>' +
        '<button class="vd-passo" data-acao="parcelas" data-d="1" aria-label="Mais uma parcela">+</button></div>';
    }
    return h;
  }

  function folhaFormas() {
    var atual = estado.rascunho.forma ? estado.rascunho.forma.codigo : null;
    return '<div class="vd-fundo" data-acao="fechar-folha"><div class="vd-folha" data-acao="nada"><h3>Forma de pagamento</h3>' +
      estado.formas.map(function (f, i) {
        return '<div class="vd-opcao' + (f.codigo === atual ? ' on' : '') + '" data-acao="forma-pick" data-i="' + i + '"><strong>' + esc(f.nome) + '</strong><small>' + esc(f.descricao) + '</small></div>';
      }).join('') + '</div></div>';
  }

  function telaEscolherCliente() {
    var h = topoComBusca('', 'cliente');
    h += '<main class="vd-corpo"><div id="vd-lista"></div></main>';
    if (estado.usuario && estado.usuario.podeLancar) {
      h += '<button class="vd-fab vd-fab-solto" data-acao="novo-cliente-abrir" aria-label="Novo cliente"><i class="fas fa-user-plus"></i></button>';
    }
    return h;
  }

  function listaClientes() {
    var busca = (document.getElementById('vd-busca') || {}).value || '';
    if (!estado.clientes.length) {
      if (estado.carregando) return '';
      return vazio(busca.trim() && estado.usuario && estado.usuario.podeLancar ? 'Nenhum cliente encontrado. Toque em + para cadastrar.' : 'Nenhum registro encontrado');
    }
    return estado.clientes.map(function (c, i) {
      var sub = [c.documento, c.telefone].filter(Boolean).join(' · ');
      return '<div class="vd-linha" data-acao="cliente-pick" data-i="' + i + '"><strong style="font-weight:600;font-size:15px">' + c.codigo + ' - ' + esc(c.nome) + '</strong>' +
        (sub ? '<div class="vd-sub">' + esc(sub) + '</div>' : '') + '</div>';
    }).join('');
  }

  function telaNovoCliente() {
    var h = topo('Novo cliente', { voltar: true });
    h += '<main class="vd-corpo com-pad">' +
      '<label class="vd-campo"><span>Nome *</span><input id="nc-nome" type="text" autocapitalize="words" autocomplete="off"></label>' +
      '<label class="vd-campo"><span>Telefone</span><input id="nc-tel" type="tel" inputmode="tel" autocomplete="off"></label>' +
      '<label class="vd-campo"><span>CPF / CNPJ</span><input id="nc-doc" type="text" inputmode="numeric" autocomplete="off"></label>' +
      '<label class="vd-campo"><span>Data de nascimento (dd/mm/aaaa)</span><input id="nc-nasc" type="text" inputmode="numeric" autocomplete="off"></label>';
    if (estado.segmentos.length) {
      h += '<div class="vd-secao">Segmento</div><div class="vd-chips" style="padding:.4rem 0">' + estado.segmentos.map(function (s) {
        return '<button class="vd-chip' + (estado.segmentosMarcados.indexOf(s.codigo) >= 0 ? ' on' : '') + '" data-acao="segmento" data-id="' + s.codigo + '">' + esc(s.nome) + '</button>';
      }).join('') + '</div>';
    }
    h += '<div style="height:1rem"></div><button id="nc-salvar" class="vd-botao" data-acao="salvar-cliente">Salvar e usar no pedido</button></main>';
    return h;
  }

  function telaItem() {
    var e = estado.edicao;
    if (!e) return null;
    var h = topo(e.indice === null ? 'Novo item' : 'Atualiza item', {
      voltar: true,
      acoes: '<button class="vd-icone" data-acao="item-salvar" aria-label="Salvar item"><i class="fas fa-check"></i></button>'
    });
    h += '<main class="vd-corpo com-pad">';
    var p = e.produto;
    h += '<label class="vd-campo"><span>Produto</span><div class="vd-falso" data-acao="produto-abrir"><span style="margin:0;color:var(--dark);font-size:16px">' +
      (p ? p.codigo + ' - ' + esc(p.nome) : '') + '</span><i class="fas fa-magnifying-glass" style="color:var(--muted)"></i></div></label>';
    h += '<div id="vd-info-produto">' + infoProduto() + '</div>';
    var temOutras = p && p.unidades && p.unidades.length > 0;
    h += '<div class="vd-grade3">' +
      '<label class="vd-campo" style="flex:.85"><span>Unidade</span><div class="vd-falso' + (temOutras ? '' : ' so-leitura') + '"' + (temOutras ? ' data-acao="unidade-abrir"' : '') + '>' +
      '<span id="vd-unidade" style="margin:0;color:var(--dark);font-size:16px">' + esc(e.unidade || (p ? p.unidade : '')) + '</span>' + (temOutras ? '<i class="fas fa-caret-down" style="color:var(--muted)"></i>' : '') + '</div></label>' +
      '<label class="vd-campo"><span>Quantidade</span><input id="it-qtd" type="text" inputmode="decimal" autocomplete="off" value="' + esc(e.quantidade) + '"></label>' +
      '<label class="vd-campo"><span>Valor unitário</span><input id="it-valor" type="text" inputmode="decimal" autocomplete="off" value="' + esc(e.valor) + '"></label></div>';
    h += '<div id="vd-erro-estoque"></div>';
    h += '<label class="vd-campo"><span>Valor total</span><div class="vd-falso so-leitura"><span id="vd-total" style="margin:0;color:var(--dark);font-size:16px">' + totalDoItem() + '</span></div></label>';
    h += '<div class="vd-secao">Dados da última venda</div><div id="vd-ultima" style="font-size:15px">' + esc(L.textoUltimaVenda(p, e.ultimaVenda)) + '</div></main>';
    if (estado.unidadeAberta && p) {
      h += '<div class="vd-fundo" data-acao="fechar-folha"><div class="vd-folha" data-acao="nada"><h3>Unidade</h3>' + L.siglas(p).map(function (s) {
        var rot = String(s).toLowerCase() === String(p.unidade).toLowerCase() ? s : s + ' (' + L.quantidade(L.fatorDe(p, s)) + ' ' + p.unidade + ')';
        return '<div class="vd-opcao' + (String(s).toLowerCase() === String(e.unidade || p.unidade).toLowerCase() ? ' on' : '') + '" data-acao="unidade-pick" data-s="' + esc(s) + '"><strong>' + esc(rot) + '</strong></div>';
      }).join('') + '</div></div>';
    }
    return h;
  }

  function infoProduto() {
    var e = estado.edicao, p = e && e.produto;
    if (!p) return '';
    var fator = L.fatorDe(p, e.unidade);
    var equivalente = fator === 1 ? '' : ' (= ' + L.quantidade(Math.floor(p.estoque / fator * 1000) / 1000) + ' ' + esc(e.unidade) + ')';
    return '<div class="vd-resumo-linha"><span style="color:var(--muted)">Preço de tabela: ' + L.moeda(L.precoDe(p, e.unidade)) + '</span>' +
      '<b class="' + (L.arredondar(p.estoque, 3) <= 0 ? 'vd-ruim' : 'vd-ok') + '" style="font-weight:600">Estoque: ' + L.quantidade(p.estoque) + ' ' + esc(p.unidade) + equivalente + '</b></div>';
  }

  function totalDoItem() {
    var e = estado.edicao;
    var q = L.lerDecimal(e.quantidade), v = L.lerDecimal(e.valor);
    return L.paraCampo(q !== null && v !== null ? q * v : 0, 2);
  }

  function pintarItem() {
    var e = estado.edicao; if (!e) return;
    var total = document.getElementById('vd-total'); if (total) total.textContent = totalDoItem();
    var passou = L.passouDoEstoque(e.produto, e.unidade || (e.produto && e.produto.unidade), e.quantidade);
    var q = document.getElementById('it-qtd'); if (q) q.classList.toggle('erro', passou);
    var aviso = document.getElementById('vd-erro-estoque');
    if (aviso) aviso.innerHTML = passou ? '<div class="vd-erro-linha">Quantidade maior que o estoque disponível.</div>' : '';
  }

  function pintarUltimaVenda() {
    var el = document.getElementById('vd-ultima');
    if (el && estado.edicao) el.textContent = L.textoUltimaVenda(estado.edicao.produto, estado.edicao.ultimaVenda);
  }

  function telaEscolherProduto() {
    var h = topoComBusca('', 'produto');
    h += '<main class="vd-corpo"><div class="vd-dica">Só aparecem produtos com estoque.</div><div id="vd-lista"></div></main>';
    return h;
  }

  function listaProdutosPedido() {
    if (!estado.produtosPedido.length) return estado.carregando ? '' : vazio();
    return estado.produtosPedido.map(function (p, i) { return linhaProduto(p, i, true); }).join('');
  }

  // ───────────────────────  desenho  ───────────────────────

  function desenhar() {
    var rota = rotaAtual();
    var html = null, depois = null;
    estado.menuAberto = estado.menuAberto && rota === '/';

    if (rota === '/' || rota === '/estoque') {
      if (estado.forcarAba) { estado.aba = estado.forcarAba; estado.forcarAba = null; history.replaceState(history.state, '', '#/'); }
      else if (rota === '/estoque' && estado.aba !== 'estoque') estado.aba = 'estoque';
      html = telaHome();
      depois = function () { atualizarLista(); recarregarHome(); };
    } else if (/^\/pedido\/\d+$/.test(rota)) {
      html = telaDetalhe();
      if (!html) { history.replaceState({ vd: 0 }, '', '#/'); return desenhar(); }
    } else if (rota === '/novo') {
      html = telaNovoPedido();
    } else if (rota === '/novo/cliente') {
      html = telaEscolherCliente();
      depois = function () { carregarClientes(''); var b = document.getElementById('vd-busca'); if (b) b.focus(); };
    } else if (rota === '/novo/cliente/novo') {
      html = telaNovoCliente();
    } else if (rota === '/novo/item') {
      html = telaItem();
      if (!html) { history.replaceState({ vd: 0 }, '', '#/novo'); return desenhar(); }
      depois = pintarItem;
    } else if (rota === '/novo/item/produto') {
      html = telaEscolherProduto();
      depois = function () { carregarProdutosParaPedido(''); var b = document.getElementById('vd-busca'); if (b) b.focus(); };
    } else {
      history.replaceState({ vd: 0 }, '', '#/'); return desenhar();
    }
    app.innerHTML = html;
    if (depois) depois();
  }

  function recarregarHome() { if (estado.aba === 'pedidos') carregarPedidos(); else carregarEstoque(); }

  function atualizarLista() {
    var el = document.getElementById('vd-lista');
    if (!el) return;
    var rota = rotaAtual();
    if (rota === '/' || rota === '/estoque') el.innerHTML = listaHome();
    else if (rota === '/novo/cliente') el.innerHTML = listaClientes();
    else if (rota === '/novo/item/produto') el.innerHTML = listaProdutosPedido();
  }

  function atualizarBotaoEnviar() {
    var b = document.getElementById('vd-enviar');
    if (b) b.disabled = !(L.pedidoPronto(estado.rascunho) && !estado.carregando);
  }

  function refazerPedido() {
    var itens = document.getElementById('vd-itens'); if (itens) itens.innerHTML = itensPedido();
    var tot = document.getElementById('vd-totais'); if (tot) tot.innerHTML = totaisPedido();
    var rod = document.getElementById('vd-rodape'); if (rod) rod.innerHTML = rodapePedido();
    atualizarBotaoEnviar();
  }

  // ───────────────────────  ações  ───────────────────────

  function novoPedido() {
    if (rascunhoTemConteudo() && !window.confirm('Você tem um pedido que ainda não foi enviado. Continuar nele?\n\nOK continua; Cancelar começa um novo.')) {
      limparRascunho();
    }
    executar(function () {
      return garantirFormas().then(function () {
        if (!estado.rascunho.forma) estado.rascunho.forma = L.formaPadrao(estado.formas);
        estado.rascunho.parcelas = L.ajustarParcelas(estado.rascunho.forma, estado.rascunho.parcelas);
        guardarRascunho();
        ir('/novo');
      });
    });
  }

  function abrirItem(indice) {
    var e = { indice: indice, produto: null, unidade: null, quantidade: '1', valor: '', ultimaVenda: null };
    if (indice !== null) {
      var i = estado.rascunho.itens[indice];
      e.produto = i.produto; e.unidade = i.unidade;
      e.quantidade = L.paraCampo(i.quantidade, 3).replace(/0+$/, '').replace(/,$/, '') || '0';
      e.valor = L.paraCampo(i.valor, 2);
    }
    estado.edicao = e; estado.unidadeAberta = false;
    ir('/novo/item');
    buscarUltimaVenda(e);
  }

  function escolherProduto(p) {
    var e = estado.edicao; if (!e) return;
    e.produto = p; e.unidade = p.unidade; e.valor = L.paraCampo(p.preco, 2); e.ultimaVenda = null;
    history.go(-1);
    buscarUltimaVenda(e);
  }

  function salvarItem() {
    var e = estado.edicao; if (!e) return;
    var r = L.validarItem(e, estado.rascunho.itens);
    if (r.erro) { aviso(r.erro, true); return; }
    if (e.indice === null) estado.rascunho.itens.push(r.item); else estado.rascunho.itens[e.indice] = r.item;
    estado.edicao = null;
    guardarRascunho();
    voltar(1, '/novo');
  }

  function salvarCliente() {
    var v = function (id) { var el = document.getElementById(id); return el ? el.value : ''; };
    var r = L.validarCliente({ nome: v('nc-nome'), documento: v('nc-doc'), telefone: v('nc-tel'), nascimento: v('nc-nasc'), segmentos: estado.segmentosMarcados.slice() });
    if (r.erro) { aviso(r.erro, true); return; }
    executar(function () {
      return api('/clientes', r.cliente).then(function (criado) {
        estado.rascunho.cliente = criado.cliente;
        guardarRascunho();
        voltar(2, '/novo');            // sai do cadastro e da escolha de cliente: volta ao pedido com o cliente já escolhido
        aviso(criado.aviso || 'Cliente cadastrado.', false);
      });
    });
  }

  function enviarPedido() {
    var r = estado.rascunho;
    if (!r.cliente) { aviso('Escolha o cliente.', true); return; }
    if (!r.forma) { aviso('Escolha a forma de pagamento.', true); return; }
    if (!r.itens.length) { aviso('Adicione pelo menos um item.', true); return; }
    executar(function () {
      return api('/pedidos', L.montarPedido(r)).then(function (criado) {
        limparRascunho();
        estado.produtos = [];
        irParaInicio();
        aviso('Pedido #' + criado.codigo + ' lançado: ' + L.moeda(criado.valor) + '.', false);
      });
    });
  }

  function abrirPedido(id) {
    executar(function () {
      return api('/pedidos/' + id).then(function (p) { estado.pedidoAberto = p; ir('/pedido/' + p.codigo); });
    });
  }

  // ───────────────────────  eventos  ───────────────────────

  var buscarHome = atrasar(function () { recarregarHome(); }, 300);
  var buscarCliente = atrasar(function (t) { carregarClientes(t); }, 300);
  var buscarProduto = atrasar(function (t) { carregarProdutosParaPedido(t); }, 300);

  app.addEventListener('click', function (ev) {
    var alvo = ev.target.closest ? ev.target.closest('[data-acao]') : null;
    if (!alvo) return;
    var a = alvo.getAttribute('data-acao');
    if (a === 'nada') { return; }
    var i = alvo.getAttribute('data-i');
    switch (a) {
      case 'aba': {
        estado.aba = alvo.getAttribute('data-aba'); estado.buscaHome = ''; estado.buscandoHome = false;
        history.replaceState(history.state, '', estado.aba === 'estoque' ? '#/estoque' : '#/'); desenhar(); break;
      }
      case 'filtro': estado.filtroEstoque = alvo.getAttribute('data-f'); desenhar(); break;
      case 'busca-toggle': estado.buscandoHome = !estado.buscandoHome; if (!estado.buscandoHome) estado.buscaHome = ''; desenhar();
        if (estado.buscandoHome) { var b = document.getElementById('vd-busca'); if (b) b.focus(); } break;
      case 'menu': estado.menuAberto = true; desenhar(); break;
      case 'menu-fechar': estado.menuAberto = false; desenhar(); break;
      case 'atualizar': estado.menuAberto = false; desenhar(); break;
      case 'novo-pedido': novoPedido(); break;
      case 'pedido-abrir': abrirPedido(alvo.getAttribute('data-id')); break;
      case 'voltar': voltar(1, rotaAtual().indexOf('/novo/') === 0 ? '/novo' : '/'); break;
      case 'escolher-cliente': ir('/novo/cliente'); break;
      case 'cliente-pick': estado.rascunho.cliente = estado.clientes[+i]; guardarRascunho(); voltar(1, '/novo'); break;
      case 'novo-cliente-abrir':
        executar(function () {
          var p = estado.segmentos.length ? Promise.resolve() : consulta('/segmentos').then(function (s) { estado.segmentos = s; });
          return p.then(function () { estado.segmentosMarcados = []; ir('/novo/cliente/novo'); });
        }); break;
      case 'segmento': {
        var id = +alvo.getAttribute('data-id'), k = estado.segmentosMarcados.indexOf(id);
        if (k >= 0) estado.segmentosMarcados.splice(k, 1); else estado.segmentosMarcados.push(id);
        alvo.classList.toggle('on', k < 0); break;
      }
      case 'salvar-cliente': salvarCliente(); break;
      case 'item-abrir':
        if (i === 'novo' && !estado.rascunho.cliente) { aviso('Escolha o cliente antes de adicionar itens.', true); break; }
        abrirItem(i === 'novo' ? null : +i); break;
      case 'item-remover': ev.stopPropagation(); estado.rascunho.itens.splice(+i, 1); guardarRascunho(); refazerPedido(); break;
      case 'produto-abrir': ir('/novo/item/produto'); break;
      case 'produto-pick': escolherProduto(estado.produtosPedido[+i]); break;
      case 'unidade-abrir': estado.unidadeAberta = true; desenhar(); break;
      case 'unidade-pick': {
        var e = estado.edicao, s = alvo.getAttribute('data-s');
        e.unidade = s; e.valor = L.paraCampo(L.precoDe(e.produto, s), 2); estado.unidadeAberta = false; desenhar(); break;
      }
      case 'item-salvar': salvarItem(); break;
      case 'forma-abrir': executar(function () { return garantirFormas().then(function () { estado.formaAberta = true; desenhar(); }); }); break;
      case 'forma-pick':
        estado.rascunho.forma = estado.formas[+i]; estado.rascunho.parcelas = 1; estado.formaAberta = false; guardarRascunho(); desenhar(); break;
      case 'fechar-folha': estado.formaAberta = false; estado.unidadeAberta = false; desenhar(); break;
      case 'parcelas':
        estado.rascunho.parcelas = L.ajustarParcelas(estado.rascunho.forma, estado.rascunho.parcelas + (+alvo.getAttribute('data-d')));
        guardarRascunho(); refazerPedido(); break;
      case 'enviar': enviarPedido(); break;
    }
  });

  app.addEventListener('input', function (ev) {
    var el = ev.target;
    if (el.id === 'vd-busca') {
      var onde = el.getAttribute('data-onde');
      if (onde === 'home') { estado.buscaHome = el.value; buscarHome(); }
      else if (onde === 'cliente') buscarCliente(el.value);
      else if (onde === 'produto') buscarProduto(el.value);
    } else if (el.id === 'it-qtd' && estado.edicao) { estado.edicao.quantidade = el.value; pintarItem(); }
    else if (el.id === 'it-valor' && estado.edicao) { estado.edicao.valor = el.value; pintarItem(); }
    else if (el.id === 'nc-nasc') { el.value = L.mascaraData(el.value); }
  });

  // ───────────────────────  início  ───────────────────────

  function iniciar() {
    // tela aberta direto numa rota interna: põe o início por baixo para o "voltar" funcionar
    var rota = rotaAtual();
    if (rota !== '/' && rota !== '/estoque') {
      history.replaceState({ vd: 0 }, '', '#/');
      history.pushState({ vd: 1 }, '', '#' + rota);
    } else if (!history.state) {
      history.replaceState({ vd: 0 }, '', location.hash || '#/');
    }
    api('/me').then(function (u) {
      estado.usuario = u;
      estado.rascunho = lerRascunho();
      desenhar();
    }, function (e) {
      if (e.status === 403) {
        app.innerHTML = '<div class="vd-vazio">' + esc(e.message) + '<br><br><a href="/dashboard" style="color:var(--silver-dark)">Abrir o painel completo</a></div>';
      } else if (e.status !== 401) {
        app.innerHTML = '<div class="vd-vazio">' + esc(e.message) + '<br><br><button class="vd-botao" style="max-width:240px;margin:0 auto" onclick="location.reload()">Tentar de novo</button></div>';
      }
    });
  }

  iniciar();
})();
