// Captura telas de REFERÊNCIA do sistema (antigo ou novo) com Playwright: desktop e celular, por perfil.
// Uso:  BASE_URL=http://localhost:8091 SAIDA=../docs/referencia node capturar-referencia.mjs
// Os dados vêm de `semear.mjs`. Salva PNGs em SAIDA/<perfil>/<viewport>/NN-nome.png e SAIDA/telas.json (status, título, erros de console).

import { chromium } from 'playwright';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const BASE = (process.env.BASE_URL || 'http://localhost:8091').replace(/\/$/, '');
const SAIDA = process.env.SAIDA || '../docs/referencia';
const SENHA = { admin: process.env.ADMIN_SENHA || 'Admin@123' };
const SENHA_TESTE = 'Teste@123';
const CANAL = process.env.PW_CANAL || undefined; // ex.: 'chromium' para o Chromium completo

const VIEWPORTS = {
  desktop: { viewport: { width: 1366, height: 900 } },
  mobile: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
};

const relatorio = [];

async function entrar(contexto, login, senha, base = BASE) {
  const pagina = await contexto.newPage();
  await pagina.goto(`${base}/login`, { waitUntil: 'load' });
  await pagina.fill('input[name="login"]', login);
  await pagina.fill('input[name="senha"]', senha);
  await Promise.all([pagina.waitForURL((u) => !/\/login/.test(u.pathname), { timeout: 15000 }), pagina.click('button[type="submit"], input[type="submit"]')]);
  return pagina;
}

async function tirar(pagina, perfil, viewport, ordem, nome, caminho, { cheia = true, espera = 700, antes } = {}) {
  const erros = [];
  const aoErro = (m) => { if (m.type() === 'error') erros.push(m.text().slice(0, 160)); };
  pagina.on('console', aoErro);
  const resp = await pagina.goto(`${BASE}${caminho}`, { waitUntil: 'load', timeout: 30000 }).catch((e) => ({ status: () => 0, erro: e.message }));
  if (antes) await antes(pagina);
  await pagina.waitForTimeout(espera);
  const pasta = join(SAIDA, perfil, viewport);
  mkdirSync(pasta, { recursive: true });
  const arquivo = `${String(ordem).padStart(2, '0')}-${nome}.png`;
  await pagina.screenshot({ path: join(pasta, arquivo), fullPage: cheia }).catch(() => {});
  pagina.off('console', aoErro);
  relatorio.push({ perfil, viewport, ordem, nome, caminho, status: resp.status?.() ?? 0, url_final: pagina.url().replace(BASE, ''), titulo: await pagina.title().catch(() => ''), erros_console: erros });
  console.log(`${perfil}/${viewport} ${arquivo} ${resp.status?.() ?? '?'} ${pagina.url().replace(BASE, '')}`);
}

async function primeiroId(pagina, caminhoLista, padrao) {
  await pagina.goto(`${BASE}${caminhoLista}`, { waitUntil: 'load' });
  const html = await pagina.content();
  const m = html.match(padrao);
  return m ? m[1] : null;
}

const navegador = await chromium.launch(CANAL ? { channel: CANAL } : {});

try {
  // ---------- anônimo: login ----------
  for (const [vp, cfg] of Object.entries(VIEWPORTS)) {
    const ctx = await navegador.newContext({ ...cfg, locale: 'pt-BR' });
    const pg = await ctx.newPage();
    await tirar(pg, 'anonimo', vp, 1, 'login', '/login');
    await tirar(pg, 'anonimo', vp, 2, 'login-erro', '/login?error=true', { cheia: false });
    await ctx.close();
  }

  // ---------- admin: todas as telas ----------
  for (const [vp, cfg] of Object.entries(VIEWPORTS)) {
    const ctx = await navegador.newContext({ ...cfg, locale: 'pt-BR' });
    const pg = await entrar(ctx, 'admin', SENHA.admin);
    const doc = await primeiroId(pg, '/documentos', /\/documentos\/visualizar\/(\d+)/);
    const docVenda = await primeiroId(pg, '/documentos?tipo=VENDA', /\/documentos\/visualizar\/(\d+)/);
    const pessoa = await primeiroId(pg, '/pessoas', /\/pessoas\/editar\/(\d+)/);
    const prod = await primeiroId(pg, '/produtos', /\/produtos\/editar\/(\d+)/);
    const tit = await primeiroId(pg, '/financeiro/titulos?status=todos', /\/financeiro\/titulos\/visualizar\/(\d+)/);
    const conta = await primeiroId(pg, '/financeiro/contas', /\/financeiro\/contas\/extrato\/(\d+)/);
    const usu = await primeiroId(pg, '/usuarios', /\/usuarios\/permissoes\/(\d+)/);
    const agd = await primeiroId(pg, '/agenda', /\/agenda\/editar\/(\d+)/);
    const mes = new Date().toISOString().slice(0, 7);
    const telas = [
      ['dashboard', '/dashboard'],
      ['produtos-lista', '/produtos'],
      ['produtos-novo', '/produtos/novo'],
      prod && ['produtos-editar', `/produtos/editar/${prod}`],
      ['unidades-lista', '/unidades'],
      ['unidades-novo', '/unidades/novo'],
      ['pessoas-lista', '/pessoas'],
      ['pessoas-nova', '/pessoas/nova'],
      pessoa && ['pessoas-editar', `/pessoas/editar/${pessoa}`],
      ['usuarios-lista', '/usuarios'],
      ['usuarios-novo', '/usuarios/novo'],
      usu && ['usuarios-permissoes', `/usuarios/permissoes/${Number(usu) + 1}`],
      ['documentos-lista', '/documentos'],
      ['documentos-novo', '/documentos/novo'],
      doc && ['documentos-visualizar', `/documentos/visualizar/${doc}`],
      docVenda && ['documentos-editar-venda', `/documentos/editar/${docVenda}`],
      ['estoque', '/estoque'],
      ['estoque-baixo', '/estoque?filtro=baixo'],
      ['relatorios-indice', '/relatorios'],
      ['relatorios-estoque', '/relatorios/estoque'],
      ['relatorios-lucratividade', '/relatorios/lucratividade'],
      ['relatorios-documentos', '/relatorios/documentos'],
      ['comissoes', '/comissoes'],
      ['metas', '/metas'],
      ['agenda-dia', '/agenda'],
      ['agenda-novo', '/agenda/novo'],
      agd && ['agenda-editar', `/agenda/editar/${agd}`],
      ['agenda-bloqueios', '/agenda/bloqueios'],
      ['agenda-horarios', '/agenda/horarios'],
      ['bi-executivo', '/bi'],
      ['bi-insights', '/bi?visao=insights'],
      ['financeiro-visao', '/financeiro'],
      ['titulos-lista', '/financeiro/titulos'],
      ['titulos-todos', '/financeiro/titulos?status=todos'],
      ['titulos-novo', '/financeiro/titulos/novo'],
      tit && ['titulos-detalhe', `/financeiro/titulos/visualizar/${tit}`],
      ['contas-lista', '/financeiro/contas'],
      ['contas-novo', '/financeiro/contas/novo'],
      conta && ['contas-extrato', `/financeiro/contas/extrato/${conta}`],
      ['contas-transferir', '/financeiro/contas/transferir'],
      ['fluxo-de-caixa', '/financeiro/fluxo'],
      ['dre-competencia', `/financeiro/dre?mes=${mes}&regime=competencia`],
      ['dre-caixa', `/financeiro/dre?mes=${mes}&regime=caixa`],
      ['formas-lista', '/financeiro/formas'],
      ['formas-novo', '/financeiro/formas/novo'],
      ['tipos-lista', '/financeiro/tipos'],
      ['categorias-lista', '/financeiro/categorias'],
      ['modo-vendedor', '/vendedor'],
      ['rota-inexistente', '/nao-existe'],
    ].filter(Boolean);
    let n = 1;
    for (const [nome, caminho] of telas) await tirar(pg, 'admin', vp, n++, nome, caminho);
    // estados de aparência (só no desktop): tema escuro e densidade compacta no dashboard
    if (vp === 'desktop') {
      await pg.evaluate(() => { localStorage.setItem('aurea-theme', 'dark'); });
      await tirar(pg, 'admin', vp, n++, 'dashboard-escuro', '/dashboard');
      await tirar(pg, 'admin', vp, n++, 'bi-escuro', '/bi');
      await pg.evaluate(() => { localStorage.setItem('aurea-theme', 'light'); localStorage.setItem('aurea-accent', 'rosa'); localStorage.setItem('aurea-density', 'compact'); });
      await tirar(pg, 'admin', vp, n++, 'dashboard-rosa-compacto', '/dashboard');
      await pg.evaluate(() => { localStorage.removeItem('aurea-accent'); localStorage.removeItem('aurea-density'); localStorage.removeItem('aurea-theme'); });
    }
    await ctx.close();
  }

  // ---------- perfis: o que cada um enxerga ----------
  const perfis = {
    vendedor1: ['dashboard', 'documentos-lista', 'bi-negado', 'modo-vendedor', 'produtos-lista', 'estoque'],
    gerente1: ['dashboard', 'bi-executivo', 'financeiro-visao', 'permissoes-negado', 'comissoes'],
    salao1: ['dashboard', 'agenda-dia', 'comissoes', 'financeiro-negado', 'documentos-lista'],
    recepcao1: ['dashboard', 'agenda-dia', 'agenda-horarios', 'documentos-negado', 'pessoas-lista'],
  };
  const rotas = {
    dashboard: '/dashboard', 'documentos-lista': '/documentos', 'bi-negado': '/bi', 'modo-vendedor': '/vendedor', 'produtos-lista': '/produtos', estoque: '/estoque',
    'bi-executivo': '/bi', 'financeiro-visao': '/financeiro', 'permissoes-negado': '/usuarios/permissoes/1', comissoes: '/comissoes',
    'agenda-dia': '/agenda', 'financeiro-negado': '/financeiro/titulos', 'agenda-horarios': '/agenda/horarios', 'documentos-negado': '/documentos', 'pessoas-lista': '/pessoas',
  };
  for (const [login, lista] of Object.entries(perfis)) {
    for (const [vp, cfg] of Object.entries(VIEWPORTS)) {
      if (vp === 'mobile' && login !== 'vendedor1') continue; // celular só para o vendedor
      const ctx = await navegador.newContext({ ...cfg, locale: 'pt-BR' });
      const pg = await entrar(ctx, login, SENHA_TESTE);
      let n = 1;
      for (const nome of lista) await tirar(pg, login, vp, n++, nome, rotas[nome]);
      await ctx.close();
    }
  }
} finally {
  await navegador.close();
  mkdirSync(SAIDA, { recursive: true });
  writeFileSync(join(SAIDA, 'telas.json'), JSON.stringify({ base: BASE, geradoEm: new Date().toISOString(), telas: relatorio }, null, 2));
  console.log(`\n${relatorio.length} capturas; relatório em ${join(SAIDA, 'telas.json')}`);
}
