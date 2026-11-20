// Semeia um conjunto de dados de REFERÊNCIA usando só as telas e a API (como um usuário faria).
// Serve para: (1) capturar telas/respostas do sistema ANTIGO; (2) rodar o MESMO roteiro no sistema NOVO e comparar.
// Uso:  BASE_URL=http://localhost:8091 node semear.mjs
// Só para bancos descartáveis de teste. Senhas abaixo são valores de TESTE (nunca usar em produção).

import { Cliente, iso, somarDias } from './lib/cliente.mjs';

const BASE = process.env.BASE_URL || 'http://localhost:8091';
const ADMIN_SENHA = process.env.ADMIN_SENHA || 'Admin@123';
const SENHA_TESTE = 'Teste@123';

const hoje = new Date();
const log = [];
let falhas = 0;

function passo(nome, r, esperado = true) {
  const msg = r && r.flash ? (r.flash.erro ? `ERRO: ${r.flash.erro}` : r.flash.sucesso || r.flash.info || '') : '';
  const ok = esperado ? !(r && r.flash && r.flash.erro) : true;
  if (!ok) falhas++;
  log.push(`${ok ? 'ok   ' : 'FALHA'} ${nome}${msg ? ' — ' + msg : ''}`);
  console.log(`${ok ? 'ok   ' : 'FALHA'} ${nome}${msg ? ' — ' + msg : ''}`);
  return r;
}

/** Caso em que o sistema DEVE recusar: sucesso = apareceu a mensagem de erro. */
function barrado(nome, r) {
  const ok = !!(r && r.flash && r.flash.erro);
  if (!ok) falhas++;
  const msg = ok ? ` — recusado: ${r.flash.erro}` : ' — NÃO foi recusado';
  log.push(`${ok ? 'ok   ' : 'FALHA'} ${nome}${msg}`);
  console.log(`${ok ? 'ok   ' : 'FALHA'} ${nome}${msg}`);
  return r;
}

const adm = new Cliente(BASE);
if (!(await adm.entrar('admin', ADMIN_SENHA))) {
  console.error('Não consegui entrar como admin. Senha/URL corretas? Bloqueio de login (15 min)?');
  process.exit(1);
}

// ---------- ids de apoio (lidos das telas) ----------
const formPessoa = await adm.pagina('/pessoas/nova');
const segs = [...formPessoa.texto.matchAll(/name="segmentoIds"[^>]*value="(\d+)"[^>]*>\s*(?:<[^>]*>\s*)*([^<\n]+)/g)].map((m) => ({ id: m[1], nome: m[2].trim() }));
const SEG_LOJA = (segs.find((s) => /loja/i.test(s.nome)) || {}).id || '1';
const SEG_SALAO = (segs.find((s) => /sal/i.test(s.nome)) || {}).id || '2';

async function pessoa(nome, o = {}) {
  const r = await adm.enviar('/pessoas/salvar', {
    upsNome: nome,
    upsCpf: o.cpf || '',
    upsTelefone: o.tel || '',
    upsNascimento: o.nasc || '',
    upsCliente: o.cliente ? 'true' : undefined,
    upsFornecedor: o.fornecedor ? 'true' : undefined,
    upsVendedor: o.vendedor ? 'true' : undefined,
    upsFuncionario: o.funcionario ? 'true' : undefined,
    upsAtivo: 'true',
    segmentoIds: o.segmentos || [],
  });
  return passo(`pessoa ${nome}`, r);
}

// ---------- 1. pessoas ----------
const nasc = (diasAdiante, anos = 30) => {
  const d = somarDias(hoje, diasAdiante);
  return `${d.getFullYear() - anos}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
};
await pessoa('Ana Profissional', { cpf: '111.222.333-96', tel: '(17) 99111-0001', vendedor: true, funcionario: true, segmentos: [SEG_SALAO], nasc: nasc(40, 28) });
await pessoa('Bia Profissional', { cpf: '222.333.444-05', tel: '(17) 99111-0002', vendedor: true, funcionario: true, segmentos: [SEG_SALAO], nasc: nasc(80, 33) });
await pessoa('Cliente Aniversariante Hoje', { cpf: '333.444.555-14', tel: '(17) 99222-0001', cliente: true, segmentos: [SEG_LOJA], nasc: nasc(0, 30) });
await pessoa('Cliente Aniversario Em Tres Dias', { cpf: '444.555.666-23', tel: '(17) 99222-0002', cliente: true, segmentos: [SEG_SALAO], nasc: nasc(3, 41) });
await pessoa('Marta Cliente Salao', { cpf: '555.666.777-32', tel: '(17) 99222-0003', cliente: true, segmentos: [SEG_SALAO, SEG_LOJA] });
const dup = await adm.enviar('/pessoas/salvar', { upsNome: 'CPF Repetido', upsCpf: '555.666.777-32', upsCliente: 'true', upsAtivo: 'true' });
barrado('pessoa com CPF repetido é barrada', dup);

const docNovo = await adm.pagina('/documentos/novo');
const pessoas = Object.fromEntries(Cliente.opcoes(docNovo.texto, 'pessoa.upsCodigo').map((o) => [o.texto, o.valor]));
const idPessoa = (parte) => {
  const k = Object.keys(pessoas).find((n) => n.toLowerCase().includes(parte.toLowerCase()));
  if (!k) throw new Error(`pessoa não encontrada: ${parte}`);
  return pessoas[k];
};
const FORNEC = idPessoa('Joias Premium');
const CLI_MARIA = idPessoa('Maria Clara');
const CLI_HOJE = idPessoa('Aniversariante Hoje');
const CLI_MARTA = idPessoa('Marta');
const CARLOS = idPessoa('Carlos Eduardo');
const ANA = idPessoa('Ana Profissional');
const BIA = idPessoa('Bia Profissional');

// ---------- 2. usuários e perfis ----------
async function usuario(nome, login, perfil, vendedorId) {
  const r = await adm.enviar('/usuarios/salvar', { usuNome: nome, usuLogin: login, novaSenha: SENHA_TESTE, perfil, vendedorId: vendedorId || '', usuAtivo: 'true' });
  return passo(`usuário ${login} (${perfil})`, r);
}
await usuario('Vendedor Loja', 'vendedor1', 'VENDEDOR_LOJA', CARLOS);
await usuario('Gerente', 'gerente1', 'GERENCIA', '');
await usuario('Profissional Salao', 'salao1', 'PROFISSIONAL_SALAO', ANA);
await usuario('Recepcao', 'recepcao1', 'RECEPCAO', '');

// ---------- 3. produtos e serviços ----------
async function produto(o) {
  const r = await adm.enviar('/produtos/salvar', {
    uprNome: o.nome, uprComplemento: o.compl || '', uprUnd: o.und || 'UN', uprPreco: o.preco, uprAtivo: 'true',
    uprServico: o.servico ? 'true' : undefined, uprComissaoPerc: o.comissao ?? '', uprDuracaoMin: o.duracao ?? '',
    segmentoId: o.seg || SEG_LOJA, unSiglas: o.un ? [o.un.sigla] : [], unFatores: o.un ? [o.un.fator] : [], unPrecos: o.un ? [o.un.preco ?? ''] : [],
  });
  return passo(`produto ${o.nome}`, r);
}
passo('unidade CX (caixa)', await adm.enviar('/unidades/salvar', { undSigla: 'CX', undDescricao: 'Caixa', undAtivo: 'true' }));
await produto({ nome: 'Blusa Seda', compl: 'Tamanho M', preco: '120.00' });
await produto({ nome: 'Calça Jeans Flare', compl: 'Tamanho 38', preco: '180.00' });
await produto({ nome: 'Brinco Pérola', compl: 'Banhado a ouro', preco: '25.00', un: { sigla: 'CX', fator: '12', preco: '270.00' } });
await produto({ nome: 'Corte feminino', preco: '80.00', servico: true, comissao: '40', duracao: '60', seg: SEG_SALAO });
await produto({ nome: 'Manicure', preco: '45.00', servico: true, comissao: '50', duracao: '45', seg: SEG_SALAO });
await produto({ nome: 'Hidratação', preco: '120.00', servico: true, comissao: '', duracao: '90', seg: SEG_SALAO });
{
  // preço 0 volta ao formulário com a mensagem do campo (não é flash): confere pelo texto da página
  const r = await adm.enviar('/produtos/salvar', { uprNome: 'Sem preço', uprUnd: 'UN', uprPreco: '0', uprAtivo: 'true', segmentoId: SEG_LOJA });
  const msg = /Pre[çc]o deve ser maior que zero/.test(r.texto || '') || !!r.flash.erro;
  console.log(`${msg ? 'ok   ' : 'FALHA'} produto com preço 0 é barrado (status ${r.status}, ${r.loc || 'sem redirect'})`);
  if (!msg) falhas++;
}

const docNovo2 = await adm.pagina('/documentos/novo');
const prods = Object.fromEntries(Cliente.opcoes(docNovo2.texto, 'produtoIds').map((o) => [o.texto, o.valor]));
const idProd = (parte) => {
  const k = Object.keys(prods).find((n) => n.toLowerCase().includes(parte.toLowerCase()));
  if (!k) throw new Error(`produto não encontrado: ${parte}`);
  return prods[k];
};
const formas = Object.fromEntries(Cliente.opcoes(docNovo2.texto, 'fpgCodigo').map((o) => [o.texto, o.valor]));
const idForma = (parte) => {
  const k = Object.keys(formas).find((n) => n.toLowerCase().startsWith(parte.toLowerCase()));
  if (!k) throw new Error(`forma não encontrada: ${parte} em ${Object.keys(formas).join(' | ')}`);
  return formas[k];
};

// ---------- 4. documentos pela tela ----------
async function documento(o) {
  const itens = o.itens;
  const r = await adm.enviar('/documentos/salvar', {
    doctoTipo: o.tipo, doctoData: o.data || iso(hoje), 'pessoa.upsCodigo': o.pessoa, doctoClassificacao: o.classificacao || '',
    produtoIds: itens.map((i) => i.p), quantidades: itens.map((i) => i.q), valores: itens.map((i) => i.v),
    unidades: itens.map((i) => i.u || ''), profissionalIds: itens.map((i) => i.prof || ''),
    fpgCodigo: o.forma || '', parcelas: o.parcelas || '', primeiroVencimento: o.venc || '', catCodigo: '', vendedorId: o.vendedor || '',
    segmentoId: o.seg || '',
  });
  return passo(o.nome, r);
}
await documento({ nome: 'COMPRA com boleto 2x e unidade CX', tipo: 'COMPRA', pessoa: FORNEC, forma: idForma('Boleto a pagar'), parcelas: '2',
  itens: [{ p: idProd('Blusa Seda'), q: '10', v: '60.00' }, { p: idProd('Calça Jeans'), q: '8', v: '90.00' }, { p: idProd('Brinco Pérola'), q: '5', v: '120.00', u: 'CX' }] });
await documento({ nome: 'ENTRADA ajuste', tipo: 'ENTRADA', classificacao: 'AJUSTE', pessoa: FORNEC, itens: [{ p: idProd('Blusa Seda'), q: '2', v: '60.00' }] });
await documento({ nome: 'SAIDA quebra', tipo: 'SAIDA', classificacao: 'QUEBRA', pessoa: FORNEC, itens: [{ p: idProd('Blusa Seda'), q: '1', v: '60.00' }] });
barrado('SAIDA sem classificação é barrada', await adm.enviar('/documentos/salvar', {
  doctoTipo: 'SAIDA', doctoData: iso(hoje), 'pessoa.upsCodigo': FORNEC, produtoIds: [idProd('Blusa Seda')], quantidades: ['1'], valores: ['60.00'], unidades: [''], profissionalIds: [''],
}));
await documento({ nome: 'VENDA dinheiro (baixa automática)', tipo: 'VENDA', pessoa: CLI_MARIA, forma: idForma('Dinheiro'), vendedor: CARLOS,
  itens: [{ p: idProd('Blusa Seda'), q: '1', v: '120.00' }] });
await documento({ nome: 'VENDA cartão de crédito 3x', tipo: 'VENDA', pessoa: CLI_HOJE, forma: idForma('Cartão de crédito'), parcelas: '3', vendedor: CARLOS,
  itens: [{ p: idProd('Calça Jeans'), q: '2', v: '180.00' }, { p: idProd('Brinco Pérola'), q: '1', v: '270.00', u: 'CX' }] });
await documento({ nome: 'VENDA salão com comissão (PIX)', tipo: 'VENDA', pessoa: CLI_MARTA, forma: idForma('PIX'), vendedor: ANA, seg: SEG_SALAO,
  itens: [{ p: idProd('Corte feminino'), q: '1', v: '80.00', prof: ANA }, { p: idProd('Manicure'), q: '1', v: '45.00', prof: ANA }, { p: idProd('Hidratação'), q: '1', v: '120.00', prof: ANA }] });
await documento({ nome: 'VENDA crediário 2x', tipo: 'VENDA', pessoa: CLI_MARIA, forma: idForma('Crediário'), parcelas: '2', vendedor: CARLOS,
  itens: [{ p: idProd('Blusa Seda'), q: '2', v: '120.00' }] });
barrado('VENDA acima do estoque é barrada', await adm.enviar('/documentos/salvar', {
  doctoTipo: 'VENDA', doctoData: iso(hoje), 'pessoa.upsCodigo': CLI_MARIA, produtoIds: [idProd('Blusa Seda')], quantidades: ['999'], valores: ['120.00'], unidades: [''], profissionalIds: [''],
  fpgCodigo: idForma('PIX'), parcelas: '', primeiroVencimento: '', catCodigo: '', vendedorId: CARLOS, segmentoId: '',
}));

// ---------- 5. pedidos pelo aplicativo (API com token) ----------
const app = new Cliente(BASE);
const login = await app.req('/api/login', { method: 'POST', json: { login: 'vendedor1', senha: SENHA_TESTE, dispositivo: 'semear.mjs' } });
let token = null;
try { token = JSON.parse(login.texto).token; } catch { /* segue sem token */ }
if (token) {
  const auth = { authorization: `Bearer ${token}` };
  const clientes = JSON.parse((await app.req('/api/clientes?busca=maria', { headers: auth })).texto);
  const formasApi = JSON.parse((await app.req('/api/formas-pagamento', { headers: auth })).texto);
  const produtosApi = JSON.parse((await app.req('/api/produtos?estoque=com', { headers: auth })).texto);
  const pix = formasApi.find((f) => /pix/i.test(f.nome));
  const blusa = produtosApi.find((p) => /blusa/i.test(p.nome));
  const r = await app.req('/api/pedidos', { method: 'POST', headers: auth, json: { clienteId: clientes[0].codigo, formaPagamentoId: pix.codigo, parcelas: 1, itens: [{ produtoId: blusa.codigo, quantidade: 1 }] } });
  console.log(`${r.status === 201 ? 'ok   ' : 'FALHA'} pedido pela API (${r.status})`);
  if (r.status !== 201) falhas++;
} else {
  console.log('FALHA login da API'); falhas++;
}

// ---------- 6. financeiro ----------
const tit = await adm.pagina('/financeiro/titulos?status=abertos&natureza=RECEBER');
const idsReceber = [...tit.texto.matchAll(/\/financeiro\/titulos\/visualizar\/(\d+)/g)].map((m) => m[1]);
const unicos = [...new Set(idsReceber)];
const contas = Cliente.opcoes((await adm.pagina(`/financeiro/titulos/visualizar/${unicos[0] || 1}`)).texto, 'contaId');
const CONTA_BANCO = (contas.find((c) => /banco/i.test(c.texto)) || contas[0] || {}).valor || '2';
const CONTA_CAIXA = (contas.find((c) => /caixa/i.test(c.texto)) || contas[0] || {}).valor || '1';
const CONTA_MAQ = (contas.find((c) => /maquin/i.test(c.texto)) || contas[0] || {}).valor || '3';
if (unicos[0]) {
  passo('baixa parcial com juros, multa e desconto (gera taxa de cartão)', await adm.enviar(`/financeiro/titulos/baixar/${unicos[0]}`, { data: iso(hoje), contaId: CONTA_MAQ, valor: '100.00', juros: '2.00', multa: '1.00', desconto: '0.50', obs: 'Baixa de teste' }));
  passo('baixa em lote', await adm.enviar('/financeiro/titulos/baixar-lote', { titIds: unicos.slice(1, 3), data: iso(hoje), contaId: CONTA_BANCO }));
}
passo('título avulso a pagar em 3 parcelas (repetir)', await adm.enviar('/financeiro/titulos/salvar', { natureza: 'PAGAR', descricao: 'Aluguel da loja', catCodigo: Cliente.opcoes((await adm.pagina('/financeiro/titulos/novo?natureza=PAGAR')).texto, 'catCodigo').find((o) => /aluguel/i.test(o.texto))?.valor || '', valor: '1200.00', emissao: iso(hoje), competencia: iso(hoje).slice(0, 7), vencimento: iso(somarDias(hoje, 10)), parcelas: '3', modo: 'repetir', segmentoId: '' }));
passo('transferência Caixa → Banco', await adm.enviar('/financeiro/contas/transferir', { origem: CONTA_CAIXA, destino: CONTA_BANCO, data: iso(hoje), valor: '50.00', observacao: 'Depósito do caixa' }));
passo('transferência Banco → Maquininha', await adm.enviar('/financeiro/contas/transferir', { origem: CONTA_BANCO, destino: CONTA_MAQ, data: iso(hoje), valor: '10.00', observacao: 'Ajuste' }));

// ---------- 7. metas ----------
const mes = iso(hoje).slice(0, 7);
passo('metas do mês (loja toda e Carlos)', await adm.enviar('/metas/salvar', { mes, vendedorIds: ['', CARLOS], valores: ['10000.00', '3000.00'] }));

// ---------- 8. agenda ----------
const horarios = (prof) => ({ profissionalId: prof, inicios: ['09:00', '09:00', '09:00', '09:00', '09:00', '09:00', ''], fins: ['18:00', '18:00', '18:00', '18:00', '18:00', '14:00', ''], pausaInicios: ['12:00', '12:00', '12:00', '12:00', '12:00', '', ''], pausaFins: ['13:00', '13:00', '13:00', '13:00', '13:00', '', ''] });
passo('expediente da Ana', await adm.enviar('/agenda/horarios/salvar', horarios(ANA)));
passo('expediente da Bia', await adm.enviar('/agenda/horarios/salvar', horarios(BIA)));
const amanha = somarDias(hoje, 1);
passo('bloqueio do salão (almoço de equipe)', await adm.enviar('/agenda/bloqueios/salvar', { profissionalId: '', inicio: `${iso(amanha)}T15:00`, fim: `${iso(amanha)}T16:00`, motivo: 'Reunião da equipe' }));
const agd = (o) => adm.enviar('/agenda/salvar', { agdCodigo: '', clienteId: o.cliente || '', clienteNome: o.nome || '', clienteTelefone: o.tel || '', data: o.data || iso(hoje), hora: o.hora, profissionalId: o.prof, obs: o.obs || '', status: '', produtoIds: o.servicos, duracoes: o.servicos.map(() => ''), valores: o.servicos.map(() => '') });
passo('agendamento Ana 10:00 (corte)', await agd({ cliente: CLI_MARTA, hora: '10:00', prof: ANA, servicos: [idProd('Corte feminino')] }));
passo('agendamento Ana 10:30 sobreposto (aviso)', await agd({ nome: 'Cliente Avulsa Joana', tel: '(17) 99333-0001', hora: '10:30', prof: ANA, servicos: [idProd('Manicure')] }));
passo('agendamento Bia amanhã 15:30 em bloqueio (aviso)', await agd({ cliente: CLI_HOJE, data: iso(amanha), hora: '15:30', prof: BIA, servicos: [idProd('Hidratação')] }));
passo('agendamento Bia domingo sem expediente (aviso)', await agd({ cliente: CLI_MARTA, data: iso(somarDias(hoje, ((7 - hoje.getDay()) % 7) || 7)), hora: '10:00', prof: BIA, servicos: [idProd('Manicure')] }));

console.log(`\nSemeadura concluída com ${falhas} falha(s).`);
console.log(JSON.stringify({ base: BASE, ids: { SEG_LOJA, SEG_SALAO, ANA, BIA, CARLOS, FORNEC } }));
process.exit(falhas ? 2 : 0);
