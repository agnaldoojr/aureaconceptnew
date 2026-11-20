// Captura o CONTRATO REAL da API /api (e de /vendedor/api) do sistema, com casos de sucesso e de erro.
// Saída: SAIDA/api/contrato.json  (requisição, status, cabeçalhos relevantes e corpo) — oráculo dos testes de contrato.
// Uso:  BASE_URL=http://localhost:8091 SAIDA=../docs/referencia node capturar-api.mjs   (depois de `node semear.mjs`)
// Tokens e datas variáveis são substituídos por marcadores (<TOKEN>, <HOJE>, <AMANHA>...) para a comparação.

import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { Cliente, iso, somarDias } from './lib/cliente.mjs';

const BASE = (process.env.BASE_URL || 'http://localhost:8091').replace(/\/$/, '');
const SAIDA = process.env.SAIDA || '../docs/referencia';
const SENHA_TESTE = 'Teste@123';
const hoje = new Date();
const casos = [];

const redigir = (txt, token) => {
  let s = txt;
  if (token) s = s.split(token).join('<TOKEN>');
  s = s.replaceAll(iso(hoje), '<HOJE>').replaceAll(iso(somarDias(hoje, 1)), '<AMANHA>');
  return s.replace(/"expiraEm":"[^"]+"/g, '"expiraEm":"<DATAHORA>"');
};

const c = new Cliente(BASE);
let token = null;

async function chamar(nome, metodo, caminho, { corpo, headers = {}, bruto, usarToken = true, cliente = c } = {}) {
  const h = { ...headers };
  if (usarToken && token) h.authorization = `Bearer ${token}`;
  const opts = { method: metodo, headers: h };
  if (bruto !== undefined) { opts.headers['content-type'] = 'application/json'; opts.body = bruto; }
  else if (corpo !== undefined) { opts.headers['content-type'] = 'application/json'; opts.body = JSON.stringify(corpo); }
  const res = await fetch(BASE + caminho, opts);
  const texto = await res.text();
  let json;
  try { json = JSON.parse(texto); } catch { json = undefined; }
  const registro = {
    nome,
    requisicao: { metodo, caminho: redigir(caminho, token), corpo: corpo !== undefined ? JSON.parse(redigir(JSON.stringify(corpo), token)) : bruto !== undefined ? bruto : undefined, autenticada: !!(usarToken && token) },
    // corpo = JSON interpretado (comparação estrutural); corpo_bruto = texto exato do servidor (formato dos números, ordem dos campos)
    resposta: { status: res.status, content_type: res.headers.get('content-type'), corpo: json !== undefined ? JSON.parse(redigir(JSON.stringify(json), token)) : redigir(texto, token), corpo_bruto: redigir(texto, token) },
  };
  casos.push(registro);
  console.log(`${String(res.status).padEnd(3)} ${metodo.padEnd(4)} ${registro.requisicao.caminho}  — ${nome}`);
  return { status: res.status, json, texto };
}

// ---------- login ----------
await chamar('login sem corpo útil', 'POST', '/api/login', { corpo: {}, usarToken: false });
await chamar('login só com login', 'POST', '/api/login', { corpo: { login: 'vendedor1' }, usarToken: false });
await chamar('login com senha errada', 'POST', '/api/login', { corpo: { login: 'vendedor1', senha: 'errada' }, usarToken: false });
await chamar('login de usuário sem APP_VENDAS (recepcao1)', 'POST', '/api/login', { corpo: { login: 'recepcao1', senha: SENHA_TESTE }, usarToken: false });
await chamar('JSON inválido no login', 'POST', '/api/login', { bruto: '{nao e json', usarToken: false });
const lg = await chamar('login correto', 'POST', '/api/login', { corpo: { login: 'vendedor1', senha: SENHA_TESTE, dispositivo: 'contrato.mjs' }, usarToken: false });
token = lg.json && lg.json.token;
if (!token) { console.error('sem token; abortando'); process.exit(1); }

// ---------- sem token ----------
await chamar('rota protegida sem token', 'GET', '/api/me', { usarToken: false });
await chamar('rota protegida com token inválido', 'GET', '/api/me', { usarToken: false, headers: { authorization: 'Bearer token-invalido' } });

// ---------- consultas ----------
await chamar('me', 'GET', '/api/me');
const cli = await chamar('clientes (todos)', 'GET', '/api/clientes');
await chamar('clientes busca "maria"', 'GET', '/api/clientes?busca=maria');
await chamar('clientes busca sem acento "aniversario"', 'GET', '/api/clientes?busca=aniversario');
await chamar('clientes limite 2', 'GET', '/api/clientes?limite=2');
await chamar('clientes busca por CPF (só dígitos)', 'GET', '/api/clientes?busca=55566677732');
await chamar('segmentos', 'GET', '/api/segmentos');
const prods = await chamar('produtos (todos)', 'GET', '/api/produtos');
await chamar('produtos com estoque', 'GET', '/api/produtos?estoque=com');
await chamar('produtos sem estoque', 'GET', '/api/produtos?estoque=sem');
await chamar('produtos busca sem acento "calca"', 'GET', '/api/produtos?busca=calca');
await chamar('produtos limite 1', 'GET', '/api/produtos?limite=1');
const formas = await chamar('formas de pagamento', 'GET', '/api/formas-pagamento');
const brinco = (prods.json || []).find((p) => /p[eé]rola/i.test(p.nome));
const blusa = (prods.json || []).find((p) => /blusa/i.test(p.nome));
const calca = (prods.json || []).find((p) => /cal[cç]a/i.test(p.nome));
await chamar('última venda de produto com venda', 'GET', `/api/produtos/${blusa.codigo}/ultima-venda`);
await chamar('última venda de produto sem venda', 'GET', `/api/produtos/999999/ultima-venda`);
await chamar('pedidos (lista)', 'GET', '/api/pedidos');
await chamar('pedidos busca por código inexistente', 'GET', '/api/pedidos?busca=999999');

// ---------- cadastro de cliente ----------
await chamar('cliente sem nome', 'POST', '/api/clientes', { corpo: { nome: '  ' } });
await chamar('cliente com nascimento futuro', 'POST', '/api/clientes', { corpo: { nome: 'Futura', nascimento: iso(somarDias(hoje, 5)) } });
await chamar('cliente criado', 'POST', '/api/clientes', { corpo: { nome: 'Cliente Pelo App', documento: '999.888.777-66', telefone: '(17) 99444-0001', nascimento: '1990-05-10', segmentos: [1] } });
await chamar('cliente com CPF duplicado (409)', 'POST', '/api/clientes', { corpo: { nome: 'Outro Nome', documento: '999.888.777-66' } });
await chamar('cliente com telefone repetido (aviso)', 'POST', '/api/clientes', { corpo: { nome: 'Telefone Igual', telefone: '(17) 99444-0001' } });

// ---------- pedidos ----------
const pix = (formas.json || []).find((f) => /pix/i.test(f.nome));
const credito = (formas.json || []).find((f) => /cr[eé]dito/i.test(f.nome));
const maria = (cli.json || []).find((x) => /maria/i.test(x.nome));
const item = (p, q = 1, extra = {}) => ({ produtoId: p.codigo, quantidade: q, ...extra });
await chamar('pedido sem itens', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, itens: [] } });
await chamar('pedido sem cliente', 'POST', '/api/pedidos', { corpo: { formaPagamentoId: pix.codigo, itens: [item(blusa)] } });
await chamar('pedido sem forma', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, itens: [item(blusa)] } });
await chamar('pedido com cliente inexistente', 'POST', '/api/pedidos', { corpo: { clienteId: 999999, formaPagamentoId: pix.codigo, itens: [item(blusa)] } });
await chamar('pedido com data futura', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, data: iso(somarDias(hoje, 2)), formaPagamentoId: pix.codigo, itens: [item(blusa)] } });
await chamar('pedido com data de mais de 30 dias', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, data: iso(somarDias(hoje, -31)), formaPagamentoId: pix.codigo, itens: [item(blusa)] } });
await chamar('pedido com produto inexistente', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, itens: [{ produtoId: 999999, quantidade: 1 }] } });
await chamar('pedido com quantidade zero', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, itens: [item(blusa, 0)] } });
await chamar('pedido com unidade inexistente', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, itens: [item(blusa, 1, { unidade: 'XX' })] } });
await chamar('pedido com valor negativo', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, itens: [item(blusa, 1, { valor: -5 })] } });
await chamar('pedido acima do estoque', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, itens: [item(blusa, 9999)] } });
await chamar('pedido com tipo errado no JSON', 'POST', '/api/pedidos', { corpo: { clienteId: 'abc', formaPagamentoId: pix.codigo, itens: [item(blusa)] } });
const ok1 = await chamar('pedido PIX (baixa automática)', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: pix.codigo, parcelas: 1, itens: [item(blusa, 1)] } });
const ok2 = await chamar('pedido crédito 3x com unidade CX e valor informado', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: credito.codigo, parcelas: 3, itens: [item(brinco, 1, { unidade: 'CX' }), item(calca, 1, { valor: 170.5 })] } });
await chamar('pedido crédito com parcelas acima do máximo', 'POST', '/api/pedidos', { corpo: { clienteId: maria.codigo, formaPagamentoId: credito.codigo, parcelas: 99, itens: [item(blusa, 1)] } });
if (ok1.json) await chamar('detalhe do pedido', 'GET', `/api/pedidos/${ok1.json.codigo}`);
if (ok2.json) await chamar('detalhe do pedido parcelado', 'GET', `/api/pedidos/${ok2.json.codigo}`);
await chamar('detalhe de pedido inexistente', 'GET', '/api/pedidos/999999');
await chamar('produtos depois das vendas (estoque baixou)', 'GET', '/api/produtos?busca=blusa');

// ---------- limite de tentativas (429) em um login que não existe: não bloqueia os outros ----------
for (let i = 0; i < 5; i++) await chamar(`tentativa errada ${i + 1} de "bloqueio-teste"`, 'POST', '/api/login', { corpo: { login: 'bloqueio-teste', senha: 'x' }, usarToken: false });
await chamar('6ª tentativa: bloqueado (429)', 'POST', '/api/login', { corpo: { login: 'bloqueio-teste', senha: 'x' }, usarToken: false });

// ---------- logout ----------
await chamar('logout', 'POST', '/api/logout');
await chamar('token revogado não vale mais', 'GET', '/api/me');

// ---------- /vendedor/api (sessão + CSRF) ----------
const web = new Cliente(BASE);
await web.entrar('vendedor1', SENHA_TESTE);
const me = await web.req('/vendedor/api/me');
casos.push({ nome: 'vendedor/api/me com sessão', requisicao: { metodo: 'GET', caminho: '/vendedor/api/me', sessao: true }, resposta: { status: me.status, content_type: me.headers.get('content-type'), corpo: (() => { try { return JSON.parse(me.texto); } catch { return me.texto.slice(0, 200); } })() } });
const semCsrf = await web.req('/vendedor/api/pedidos', { method: 'POST', json: { clienteId: 1 } });
casos.push({ nome: 'vendedor/api POST sem CSRF', requisicao: { metodo: 'POST', caminho: '/vendedor/api/pedidos', sessao: true, csrf: false }, resposta: { status: semCsrf.status, content_type: semCsrf.headers.get('content-type'), corpo: semCsrf.texto.slice(0, 200) } });
const anon = await fetch(BASE + '/vendedor/api/me', { redirect: 'manual' });
casos.push({ nome: 'vendedor/api/me sem sessão', requisicao: { metodo: 'GET', caminho: '/vendedor/api/me', sessao: false }, resposta: { status: anon.status, location: anon.headers.get('location') } });

mkdirSync(join(SAIDA, 'api'), { recursive: true });
writeFileSync(join(SAIDA, 'api', 'contrato.json'), JSON.stringify({ base: BASE, geradoEm: new Date().toISOString(), casos }, null, 2));
console.log(`\n${casos.length} casos gravados em ${join(SAIDA, 'api', 'contrato.json')}`);
