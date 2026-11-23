// Executor dos testes GOLDEN: roda o MESMO roteiro no sistema antigo (oráculo) e no novo e compara, passo a passo,
// o status HTTP, o redirecionamento, o título e a tela (área de conteúdo normalizada). Só desenvolvimento.
//
// Um roteiro é uma função async (c, {seguir, post, passo}) que registra passos. Passo com `esperaDivergir: 'D-03'` é uma
// diferença DECIDIDA (melhoria aprovada): aparece como "≈" e não conta como divergência.
import { Cliente } from './cliente.mjs';
import { areaConteudo, normalizar, primeiraDiferenca, titulo } from './html.mjs';

const BASES = {
  antigo: { base: process.env.BASE_ANTIGO || 'http://localhost:8091', senha: process.env.SENHA_ANTIGO || 'Admin@123' },
  novo: { base: process.env.BASE_NOVO || 'http://localhost:8092', senha: process.env.SENHA_NOVO || 'Teste-Local-2026' },
};

export async function entrar(c, senha) {
  const tela = await c.req('/login');
  c.lerCsrf(tela.texto);
  const r = await c.req('/login', { method: 'POST', form: { login: 'admin', senha, modoApp: '0' } });
  if (!(r.status === 302 && /\/dashboard$/.test(r.loc || ''))) throw new Error(`login falhou: ${r.status} ${r.loc}`);
  const f = await c.req('/unidades/novo'); // qualquer formulário traz o token da sessão nova
  if (!c.lerCsrf(f.texto)) throw new Error('sem token CSRF após o login');
}

function criarRegistro(c) {
  const passos = [];
  const ajudas = {
    passos,
    passo: async (nome, fn, opcoes = {}) => {
      const r = await fn();
      passos.push({ nome, r, tela: opcoes.tela !== false, esperaDivergir: opcoes.esperaDivergir });
      return r;
    },
  };
  ajudas.seguir = (nome, caminho, opcoes) => ajudas.passo(nome, () => c.req(caminho), opcoes);
  ajudas.post = (nome, caminho, form, opcoes) => ajudas.passo(nome, () => c.req(caminho, { method: 'POST', form }), opcoes);
  // posta e, se vier redirecionamento, abre a tela de destino (onde aparece a mensagem de uma só leitura)
  ajudas.postVer = async (nome, caminho, form, opcoes) => {
    const r = await ajudas.post(nome, caminho, form, opcoes);
    if (r.status === 302 && r.loc) await ajudas.seguir(`  ↳ ${nome}: tela de destino`, new URL(r.loc, 'http://x').pathname + new URL(r.loc, 'http://x').search, opcoes);
    return r;
  };
  return ajudas;
}

async function rodarEm(nome, roteiro) {
  const { base, senha } = BASES[nome];
  const c = new Cliente(base);
  await entrar(c, senha);
  const aj = criarRegistro(c);
  await roteiro(c, aj);
  return aj.passos;
}

function resumo(p) {
  const loc = (p.r.loc || '').replace(/^https?:\/\/[^/]+/, '');
  const comTela = p.tela && p.r.status === 200;
  return { status: p.r.status, loc, titulo: comTela ? titulo(p.r.texto) : '', linhas: comTela ? normalizar(areaConteudo(p.r.texto)) : [] };
}

export async function executarGolden(roteiro) {
  const [a, n] = await Promise.all([rodarEm('antigo', roteiro), rodarEm('novo', roteiro)]);
  if (a.length !== n.length) throw new Error(`roteiros de tamanhos diferentes: ${a.length} x ${n.length}`);
  let divergencias = 0;
  let esperadas = 0;
  for (let i = 0; i < a.length; i++) {
    const x = resumo(a[i]);
    const y = resumo(n[i]);
    const problemas = [];
    if (x.status !== y.status) problemas.push(`status antigo ${x.status} ≠ novo ${y.status}`);
    if (x.loc !== y.loc) problemas.push(`redirecionamento antigo "${x.loc}" ≠ novo "${y.loc}"`);
    if (x.titulo !== y.titulo) problemas.push(`título antigo "${x.titulo}" ≠ novo "${y.titulo}"`);
    const d = primeiraDiferenca(x.linhas, y.linhas);
    if (d) problemas.push(`tela diferente, ${d}`);
    const esperaDivergir = a[i].esperaDivergir;
    if (process.env.ALERTAS) {
      const al = (p) => (p.tela && p.r.status === 200 ? ((p.r.texto.match(/<div class="alert[^>]*>[\s\S]*?<\/div>/) || [''])[0].replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()) : '');
      console.log(`   [alerta] antigo: "${al(a[i])}" | novo: "${al(n[i])}"`);
    }
    if (problemas.length && esperaDivergir) {
      esperadas++;
      console.log(`≈ ${a[i].nome}  (diferença decidida ${esperaDivergir})\n   ${problemas.join('\n   ')}`);
    } else if (problemas.length) {
      divergencias++;
      console.log(`✗ ${a[i].nome}\n   ${problemas.join('\n   ')}`);
    } else {
      if (esperaDivergir) console.log(`? ${a[i].nome}  igual, mas era esperada diferença (${esperaDivergir})`);
      console.log(`✓ ${a[i].nome}  [${x.status}${x.loc ? ' → ' + x.loc : ''}]${x.linhas.length ? `  (${x.linhas.length} linhas de tela comparadas)` : ''}`);
    }
  }
  console.log(`\n${a.length} passos, ${divergencias} divergência(s)${esperadas ? `, ${esperadas} diferença(s) decidida(s)` : ''}.`);
  process.exit(divergencias ? 1 : 0);
}
