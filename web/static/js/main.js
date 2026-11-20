/**
 * Aurea Concept — main.js
 * Utilitários globais do sistema
 */

// ── PREFERÊNCIAS (tema, destaque, densidade, menu) ─────────────
// Guardadas só neste navegador. tema-init.js aplica tudo no <head>; aqui ficam as trocas em tempo real.
var AUREA = {
  get: function (chave, padrao) {
    try { return window.localStorage.getItem('aurea-' + chave) || padrao; } catch (e) { return padrao; }
  },
  set: function (chave, valor) {
    try { window.localStorage.setItem('aurea-' + chave, valor); } catch (e) { /* sem armazenamento: vale só nesta página */ }
  }
};

var RAIZ = document.documentElement;
var mqEscuro = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
var mqCelular = window.matchMedia ? window.matchMedia('(max-width: 768px)') : { matches: false };

function temaEfetivo(pref) {
  return pref === 'dark' || (pref === 'system' && mqEscuro && mqEscuro.matches) ? 'dark' : 'light';
}

// Gráficos (Chart.js) não leem CSS: ao trocar de tema a página recarrega para redesenhá-los com as cores novas.
function recarregarSeTiverGrafico() {
  if (document.querySelector('canvas')) window.location.reload();
}

function aplicarTema(pref) {
  AUREA.set('theme', pref);
  RAIZ.setAttribute('data-theme-pref', pref);
  RAIZ.setAttribute('data-theme', temaEfetivo(pref));
  atualizarControles();
  recarregarSeTiverGrafico();
}

function alternarTema() {
  aplicarTema(RAIZ.getAttribute('data-theme') === 'dark' ? 'light' : 'dark');
}

function aplicarDestaque(valor) {
  AUREA.set('accent', valor);
  if (valor === 'prata') RAIZ.removeAttribute('data-accent'); else RAIZ.setAttribute('data-accent', valor);
  atualizarControles();
  recarregarSeTiverGrafico();
}

function aplicarDensidade(valor) {
  AUREA.set('density', valor);
  RAIZ.setAttribute('data-density', valor);
  atualizarControles();
}

// "Automático" acompanha o tema do sistema operacional em tempo real
if (mqEscuro && mqEscuro.addEventListener) {
  mqEscuro.addEventListener('change', function () {
    if (RAIZ.getAttribute('data-theme-pref') === 'system') {
      RAIZ.setAttribute('data-theme', temaEfetivo('system'));
      atualizarControles();
      recarregarSeTiverGrafico();
    }
  });
}

// Marca o botão ativo de cada grupo e o ícone do botão rápido de tema
function atualizarControles() {
  var atual = {
    theme: RAIZ.getAttribute('data-theme-pref') || 'light',
    accent: RAIZ.getAttribute('data-accent') || 'prata',
    density: RAIZ.getAttribute('data-density') || 'comfortable'
  };
  document.querySelectorAll('[data-pref]').forEach(function (grupo) {
    var pref = grupo.getAttribute('data-pref');
    grupo.querySelectorAll('button').forEach(function (b) {
      var ligado = b.getAttribute('data-valor') === atual[pref];
      b.classList.toggle('on', ligado);
      b.setAttribute('aria-pressed', ligado ? 'true' : 'false');
    });
  });
  var btn = document.getElementById('btnTema');
  if (btn) {
    var escuro = RAIZ.getAttribute('data-theme') === 'dark';
    btn.innerHTML = '<i class="fas ' + (escuro ? 'fa-sun' : 'fa-moon') + '"></i>';
    btn.title = escuro ? 'Mudar para o tema claro' : 'Mudar para o tema escuro';
  }
}

document.addEventListener('click', function (e) {
  var b = e.target.closest ? e.target.closest('[data-pref] button') : null;
  if (!b) return;
  var pref = b.parentElement.getAttribute('data-pref');
  var valor = b.getAttribute('data-valor');
  if (pref === 'theme') aplicarTema(valor);
  else if (pref === 'accent') aplicarDestaque(valor);
  else if (pref === 'density') aplicarDensidade(valor);
});

// ── MENU LATERAL ─────────────────────────────────────────────
// Celular: abre/fecha como gaveta. Computador: recolhe para só ícones (e lembra a escolha).
function toggleSidebar() {
  var sidebar = document.getElementById('sidebar');
  if (!sidebar) return;
  if (mqCelular.matches) {
    var aberto = sidebar.classList.toggle('open');
    document.body.classList.toggle('nav-open', aberto);
  } else {
    var recolhido = RAIZ.classList.toggle('sidebar-collapsed');
    AUREA.set('sidebar', recolhido ? 'collapsed' : 'open');
  }
}

// Fecha o menu ao clicar fora (celular)
document.addEventListener('click', function (e) {
  var sidebar = document.getElementById('sidebar');
  var toggle  = document.querySelector('.sidebar-toggle');
  if (!sidebar || !toggle) return;

  if (sidebar.classList.contains('open')
    && !sidebar.contains(e.target)
    && !toggle.contains(e.target)) {
    sidebar.classList.remove('open');
    document.body.classList.remove('nav-open');
  }
});

// Grupos do menu (Loja de roupa, Salão, Gerência) recolhíveis; o grupo da tela atual nunca fica fechado.
function iniciarGruposDoMenu() {
  var itens = document.querySelectorAll('.sidebar-menu > li');
  var secao = null, filhos = [];

  function fechar() {
    if (!secao) return;
    var chave = 'grp-' + secao.getAttribute('data-nome');
    var temAtivo = filhos.some(function (li) { return li.classList.contains('active'); });
    var fechado = !temAtivo && AUREA.get(chave, 'aberto') === 'fechado';
    secao.classList.toggle('fechado', fechado);
    secao.setAttribute('aria-expanded', fechado ? 'false' : 'true');
    filhos.forEach(function (li) { li.classList.toggle('grupo-fechado', fechado); });
  }

  itens.forEach(function (li) {
    if (li.classList.contains('sidebar-section')) {
      secao = li; filhos = [];
      var nome = li.textContent.trim();
      li.setAttribute('data-nome', nome);
      li.setAttribute('role', 'button');
      li.setAttribute('tabindex', '0');
      li.innerHTML = '<span>' + nome + '</span><i class="fas fa-chevron-down sec-caret"></i>';
      var alternar = (function (s) {
        return function () {
          var cs = s.classList.contains('fechado') ? 'aberto' : 'fechado';
          AUREA.set('grp-' + s.getAttribute('data-nome'), cs);
          iniciarGruposDoMenu();
        };
      })(li);
      li.onclick = alternar;
      li.onkeydown = function (e) { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); alternar(); } };
    } else if (secao) {
      filhos.push(li);
    }
    // ao fechar o último filho de um grupo, aplica o estado do grupo
    var proximo = li.nextElementSibling;
    if (secao && (!proximo || proximo.classList.contains('sidebar-section'))) fechar();
  });

  // tooltip nativo nos ícones (útil com o menu recolhido)
  document.querySelectorAll('.sidebar-menu li a').forEach(function (a) {
    var t = a.textContent.trim();
    if (t) a.title = t;
  });
}

// ── MENU DO USUÁRIO ──────────────────────────────────────────
function alternarUserMenu(e) {
  if (e) e.stopPropagation();
  var drop = document.querySelector('.user-dropdown');
  var btn = document.querySelector('.user-trigger');
  if (!drop || !btn) return;
  var abrir = drop.hasAttribute('hidden');
  if (abrir) drop.removeAttribute('hidden'); else drop.setAttribute('hidden', '');
  btn.setAttribute('aria-expanded', abrir ? 'true' : 'false');
}

function fecharUserMenu() {
  var drop = document.querySelector('.user-dropdown');
  var btn = document.querySelector('.user-trigger');
  if (drop) drop.setAttribute('hidden', '');
  if (btn) btn.setAttribute('aria-expanded', 'false');
}

document.addEventListener('click', function (e) {
  var menu = document.getElementById('userMenu');
  if (menu && !menu.contains(e.target)) fecharUserMenu();
});

function iniciarIniciais() {
  var nome = (document.querySelector('.user-trigger .user-name') || {}).textContent || '';
  nome = nome.trim();
  var partes = nome.split(/[\s._-]+/).filter(Boolean);
  var ini = partes.length > 1 ? partes[0][0] + partes[1][0] : nome.slice(0, 2);
  document.querySelectorAll('.js-iniciais').forEach(function (el) { el.textContent = (ini || '?').toUpperCase(); });
}

// ── PALETA DE COMANDOS (Ctrl+K) ──────────────────────────────
// Lista as telas do menu (já filtradas pelas permissões do usuário) e algumas ações de aparência.
var paleta = { itens: [], visiveis: [], sel: 0 };

function semAcento(s) {
  return (s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
}

function montarItensDaPaleta() {
  var lista = [];
  var grupo = 'Geral';
  document.querySelectorAll('.sidebar-menu > li').forEach(function (li) {
    if (li.classList.contains('sidebar-section')) { grupo = li.getAttribute('data-nome') || li.textContent.trim(); return; }
    var a = li.querySelector('a');
    if (!a) return;
    var icone = a.querySelector('i');
    lista.push({
      grupo: grupo === 'Geral' ? 'Telas' : grupo,
      nome: (a.querySelector('span') || a).textContent.trim(),
      icone: icone ? icone.className : 'fas fa-circle',
      acao: (function (h) { return function () { window.location.href = h; }; })(a.getAttribute('href'))
    });
  });
  var acoes = [
    ['Tema claro', 'fas fa-sun', function () { aplicarTema('light'); }],
    ['Tema escuro', 'fas fa-moon', function () { aplicarTema('dark'); }],
    ['Tema automático (segue o sistema)', 'fas fa-circle-half-stroke', function () { aplicarTema('system'); }],
    ['Cor de destaque: prata', 'fas fa-palette', function () { aplicarDestaque('prata'); }],
    ['Cor de destaque: azul', 'fas fa-palette', function () { aplicarDestaque('azul'); }],
    ['Cor de destaque: verde-água', 'fas fa-palette', function () { aplicarDestaque('verde'); }],
    ['Cor de destaque: rosa', 'fas fa-palette', function () { aplicarDestaque('rosa'); }],
    ['Densidade compacta', 'fas fa-compress', function () { aplicarDensidade('compact'); }],
    ['Densidade confortável', 'fas fa-border-all', function () { aplicarDensidade('comfortable'); }],
    ['Densidade espaçosa', 'fas fa-expand', function () { aplicarDensidade('spacious'); }],
    ['Recolher ou expandir o menu', 'fas fa-bars', function () { toggleSidebar(); }]
  ];
  acoes.forEach(function (a) { lista.push({ grupo: 'Aparência', nome: a[0], icone: a[1], acao: a[2] }); });
  var sair = document.querySelector('.ud-logout');
  if (sair) lista.push({ grupo: 'Conta', nome: 'Sair do sistema', icone: 'fas fa-sign-out-alt',
    acao: function () { window.location.href = sair.getAttribute('href'); } });
  return lista;
}

function desenharPaleta() {
  var termo = semAcento((document.getElementById('paletaBusca') || {}).value || '').trim();
  var palavras = termo ? termo.split(/\s+/) : [];
  paleta.visiveis = paleta.itens.filter(function (it) {
    var alvo = semAcento(it.nome + ' ' + it.grupo);
    return palavras.every(function (p) { return alvo.indexOf(p) >= 0; });
  });
  if (paleta.sel >= paleta.visiveis.length) paleta.sel = 0;
  var ul = document.getElementById('paletaLista');
  if (!ul) return;
  if (!paleta.visiveis.length) { ul.innerHTML = '<li class="palette-vazio">Nada encontrado para essa busca.</li>'; return; }
  var html = '', ultimo = null;
  paleta.visiveis.forEach(function (it, i) {
    if (it.grupo !== ultimo) { html += '<li class="palette-group-title">' + it.grupo + '</li>'; ultimo = it.grupo; }
    html += '<li class="palette-item' + (i === paleta.sel ? ' sel' : '') + '" role="option" data-i="' + i + '">' +
            '<i class="' + it.icone + '"></i><span>' + it.nome + '</span></li>';
  });
  ul.innerHTML = html;
  var sel = ul.querySelector('.sel');
  if (sel && sel.scrollIntoView) sel.scrollIntoView({ block: 'nearest' });
}

function abrirPaleta() {
  var el = document.getElementById('paleta');
  if (!el) return;
  fecharUserMenu();
  paleta.itens = montarItensDaPaleta();
  paleta.sel = 0;
  el.removeAttribute('hidden');
  var campo = document.getElementById('paletaBusca');
  campo.value = '';
  desenharPaleta();
  campo.focus();
}

function fecharPaleta() {
  var el = document.getElementById('paleta');
  if (el) el.setAttribute('hidden', '');
}

function executarDaPaleta(i) {
  var it = paleta.visiveis[i];
  if (!it) return;
  fecharPaleta();
  it.acao();
}

document.addEventListener('keydown', function (e) {
  if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault();
    var aberta = document.getElementById('paleta') && !document.getElementById('paleta').hasAttribute('hidden');
    if (aberta) fecharPaleta(); else abrirPaleta();
    return;
  }
  var el = document.getElementById('paleta');
  if (!el || el.hasAttribute('hidden')) {
    if (e.key === 'Escape') fecharUserMenu();
    return;
  }
  if (e.key === 'Escape') { e.preventDefault(); fecharPaleta(); }
  else if (e.key === 'ArrowDown') { e.preventDefault(); paleta.sel = Math.min(paleta.sel + 1, paleta.visiveis.length - 1); desenharPaleta(); }
  else if (e.key === 'ArrowUp') { e.preventDefault(); paleta.sel = Math.max(paleta.sel - 1, 0); desenharPaleta(); }
  else if (e.key === 'Enter') { e.preventDefault(); executarDaPaleta(paleta.sel); }
});

document.addEventListener('input', function (e) {
  if (e.target && e.target.id === 'paletaBusca') { paleta.sel = 0; desenharPaleta(); }
});

document.addEventListener('click', function (e) {
  var item = e.target.closest ? e.target.closest('.palette-item') : null;
  if (item) executarDaPaleta(parseInt(item.getAttribute('data-i'), 10));
});

// ── GRÁFICOS (Chart.js): cores vêm do tema atual ─────────────
window.aureaCores = function () {
  var cs = getComputedStyle(RAIZ);
  function v(nome, padrao) { return (cs.getPropertyValue(nome) || '').trim() || padrao; }
  return {
    texto: v('--mid', '#555'),
    grade: v('--border-light', '#e5e5e5'),
    superficie: v('--bg-white', '#fff'),
    s1: v('--chart-1', '#8a8d92'),
    s2: v('--chart-2', '#55585d'),
    s3: v('--chart-3', '#2b2c2f'),
    s4: v('--chart-4', '#c3c5c8'),
    fonte: v('--font-body', 'system-ui, sans-serif')
  };
};

window.aureaGraficos = function () {
  if (!window.Chart) return;
  var c = window.aureaCores();
  Chart.defaults.color = c.texto;
  Chart.defaults.borderColor = c.grade;
  Chart.defaults.font.family = c.fonte;
};

// ── APP INSTALÁVEL (PWA) ─────────────────────────────────────
// No iPhone/iPad não existe botão "instalar": a dica no menu do usuário só aparece em quem está no
// navegador (Safari) e ainda não instalou o app na tela de início.
function iniciarDicaDeInstalacao() {
  var bloco = document.getElementById('dicaInstalar');
  if (!bloco) return;
  var ua = navigator.userAgent || '';
  var ios = /iPad|iPhone|iPod/.test(ua) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
  var instalado = window.navigator.standalone === true ||
    (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches);
  if (ios && !instalado) bloco.removeAttribute('hidden');
}

document.addEventListener('DOMContentLoaded', function () {
  iniciarGruposDoMenu();
  iniciarIniciais();
  atualizarControles();
  iniciarDicaDeInstalacao();
});

// ── ALERTAS AUTO-DISMISS ─────────────────────────────────────
document.addEventListener('DOMContentLoaded', function () {
  const alerts = document.querySelectorAll('.alert');
  alerts.forEach(function (alert) {
    setTimeout(function () {
      alert.style.transition = 'opacity .5s ease, max-height .5s ease';
      alert.style.opacity    = '0';
      alert.style.maxHeight  = '0';
      alert.style.overflow   = 'hidden';
      setTimeout(function () { alert.remove(); }, 550);
    }, 5000);
  });
});

// ── CONFIRMAÇÃO DE EXCLUSÃO ─────────────────────────────────
// Uso: <form onsubmit="return confirmar('Excluir produto?')">
function confirmar(mensagem) {
  return window.confirm(mensagem || 'Confirma esta ação?');
}

// ── TOGGLE VISIBILIDADE DE SENHA ─────────────────────────────
function toggleSenha(inputId, btn) {
  const input = document.getElementById(inputId);
  const icon  = btn.querySelector('i');
  if (!input) return;

  if (input.type === 'password') {
    input.type   = 'text';
    icon.className = 'fas fa-eye-slash';
  } else {
    input.type   = 'password';
    icon.className = 'fas fa-eye';
  }
}

// ── FORMATAÇÃO DE MOEDA ──────────────────────────────────────
function formatBRL(value) {
  return new Intl.NumberFormat('pt-BR', {
    style:                 'currency',
    currency:              'BRL',
    minimumFractionDigits: 2
  }).format(value || 0);
}

// ── DOCUMENTOS — ITENS DINÂMICOS ─────────────────────────────
// (Complemento ao JS inline da página de documento)

/**
 * Calcula e atualiza o total de todos os itens na tabela de documento.
 */
function recalcularTotalGeral() {
  let total = 0;
  document.querySelectorAll('.item-row').forEach(function (row) {
    const qtd = parseFloat(row.querySelector('.qtd-input')?.value) || 0;
    const vlr = parseFloat(row.querySelector('.vlr-input')?.value) || 0;
    const rowTotal = qtd * vlr;

    const totalEl = row.querySelector('.total-item');
    if (totalEl) totalEl.textContent = formatBRL(rowTotal);

    total += rowTotal;
  });

  const totalGeralEl = document.getElementById('totalGeral');
  if (totalGeralEl) totalGeralEl.textContent = formatBRL(total);
}

// ── MÁSCARA CPF/CNPJ ────────────────────────────────────────
function mascaraCpfCnpj(input) {
  let v = input.value.replace(/\D/g, '');

  if (v.length <= 11) {
    // CPF: 000.000.000-00
    v = v.replace(/(\d{3})(\d)/, '$1.$2');
    v = v.replace(/(\d{3})(\d)/, '$1.$2');
    v = v.replace(/(\d{3})(\d{1,2})$/, '$1-$2');
  } else {
    // CNPJ: 00.000.000/0000-00
    v = v.replace(/^(\d{2})(\d)/, '$1.$2');
    v = v.replace(/^(\d{2})\.(\d{3})(\d)/, '$1.$2.$3');
    v = v.replace(/\.(\d{3})(\d)/, '.$1/$2');
    v = v.replace(/(\d{4})(\d)/, '$1-$2');
  }

  input.value = v;
}

// ── MÁSCARA TELEFONE ─────────────────────────────────────────
function mascaraTelefone(input) {
  let v = input.value.replace(/\D/g, '');
  if (v.length <= 10) {
    v = v.replace(/(\d{2})(\d)/, '($1) $2');
    v = v.replace(/(\d{4})(\d)/, '$1-$2');
  } else {
    v = v.replace(/(\d{2})(\d)/, '($1) $2');
    v = v.replace(/(\d{5})(\d)/, '$1-$2');
  }
  input.value = v;
}

// ── APLICAR MÁSCARAS AUTOMATICAMENTE ─────────────────────────
document.addEventListener('DOMContentLoaded', function () {
  // CPF/CNPJ
  const cpfInput = document.querySelector('input[name="upsCpf"]');
  if (cpfInput) {
    cpfInput.addEventListener('input', function () { mascaraCpfCnpj(this); });
  }

  // Telefone
  const telInput = document.querySelector('input[name="upsTelefone"]');
  if (telInput) {
    telInput.addEventListener('input', function () { mascaraTelefone(this); });
  }

  // Cálculo de total no formulário de documento
  document.querySelectorAll('.qtd-input, .vlr-input').forEach(function (el) {
    el.addEventListener('input', recalcularTotalGeral);
  });
});

// ── FEEDBACK VISUAL EM BOTÕES SUBMIT ─────────────────────────
document.addEventListener('DOMContentLoaded', function () {
  document.querySelectorAll('form').forEach(function (form) {
    form.addEventListener('submit', function () {
      const btn = form.querySelector('button[type="submit"]');
      if (btn && !btn.disabled) {
        btn.disabled = true;
        const original = btn.innerHTML;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Aguarde...';
        setTimeout(function () {
          btn.disabled = false;
          btn.innerHTML = original;
        }, 8000);
      }
    });
  });
});

// ── TOOLTIPS SIMPLES ──────────────────────────────────────────
document.addEventListener('DOMContentLoaded', function () {
  document.querySelectorAll('[title]').forEach(function (el) {
    // Mantém o title nativo (navegadores modernos já exibem)
  });
});
