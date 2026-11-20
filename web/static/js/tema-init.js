/**
 * Aurea Concept — tema-init.js
 * Roda no <head>, antes do CSS pintar a tela, para o tema escolhido (claro/escuro, cor de destaque,
 * densidade e menu recolhido) já estar aplicado e a página não "piscar" no carregamento.
 * As preferências ficam só neste navegador (localStorage, chaves "aurea-...").
 */
(function () {
  var d = document.documentElement;

  function ler(chave, validos, padrao) {
    var v = null;
    try { v = window.localStorage.getItem('aurea-' + chave); } catch (e) { /* navegação privada: usa o padrão */ }
    return validos.indexOf(v) >= 0 ? v : padrao;
  }

  var pref = ler('theme', ['light', 'dark', 'system'], 'light');
  var escuro = pref === 'dark' ||
    (pref === 'system' && window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
  d.setAttribute('data-theme', escuro ? 'dark' : 'light');
  d.setAttribute('data-theme-pref', pref);

  var destaque = ler('accent', ['prata', 'azul', 'verde', 'rosa'], 'prata');
  if (destaque !== 'prata') d.setAttribute('data-accent', destaque);

  d.setAttribute('data-density', ler('density', ['compact', 'comfortable', 'spacious'], 'comfortable'));

  if (ler('sidebar', ['collapsed', 'open'], 'open') === 'collapsed') d.classList.add('sidebar-collapsed');

  // App instalado: quem é vendedor cai no Modo vendedor ao abrir o ícone (uma vez por abertura do app).
  // Só a tela inicial traz a marca; o navegador comum e o computador nunca são redirecionados.
  try {
    var instalado = window.navigator.standalone === true ||
      (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches);
    var marca = document.querySelector('meta[name="aurea-entrada"]');
    if (instalado && marca && marca.getAttribute('content') === 'vendedor' &&
        !window.sessionStorage.getItem('aurea-entrada')) {
      window.sessionStorage.setItem('aurea-entrada', '1');
      window.location.replace('/vendedor');
    }
  } catch (e) { /* sem armazenamento: não redireciona, abre o painel normal */ }
})();
