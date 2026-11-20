// Teste GOLDEN dos cadastros (Unidades e Pessoas): roda o MESMO roteiro no sistema antigo (oráculo) e no novo e compara,
// passo a passo, o status HTTP, o redirecionamento e a tela (área de conteúdo normalizada).
//   node comparar-cadastros.mjs                        (antigo em :8091 admin/Admin@123, novo em :8092 admin/SENHA_NOVO)
// Variáveis: BASE_ANTIGO, BASE_NOVO, SENHA_ANTIGO, SENHA_NOVO. Só desenvolvimento.
import { Cliente } from './lib/cliente.mjs';
import { areaConteudo, normalizar, primeiraDiferenca, titulo } from './lib/html.mjs';

const BASES = {
  antigo: { base: process.env.BASE_ANTIGO || 'http://localhost:8091', senha: process.env.SENHA_ANTIGO || 'Admin@123' },
  novo: { base: process.env.BASE_NOVO || 'http://localhost:8092', senha: process.env.SENHA_NOVO || 'Teste-Local-2026' },
};

async function entrar(c, senha) {
  const tela = await c.req('/login');
  c.lerCsrf(tela.texto);
  const r = await c.req('/login', { method: 'POST', form: { login: 'admin', senha, modoApp: '0' } });
  if (!(r.status === 302 && /\/dashboard$/.test(r.loc || ''))) throw new Error(`login falhou: ${r.status} ${r.loc}`);
  const f = await c.req('/unidades/novo'); // qualquer formulário traz o token da sessão nova
  if (!c.lerCsrf(f.texto)) throw new Error('sem token CSRF após o login');
}

/** O roteiro. Cada passo devolve {nome, r}; "tela" marca os passos cuja página também é comparada. */
async function roteiro(c) {
  const passos = [];
  const rodar = async (nome, fn, { tela = true } = {}) => {
    const r = await fn();
    passos.push({ nome, r, tela });
    return r;
  };
  const seguir = async (nome, caminho) => rodar(nome, () => c.req(caminho));
  const post = (nome, caminho, form, opts) => rodar(nome, () => c.req(caminho, { method: 'POST', form }), opts);
  const un = (extra = {}) => ({ undCodigo: '', undSigla: 'CX2', undDescricao: 'Caixa dois', undAtivo: 'true', _undAtivo: 'on', ...extra });

  // ───────── Unidades ─────────
  await seguir('unidades: lista inicial', '/unidades');
  await seguir('unidades: formulário novo', '/unidades/novo');
  await post('unidades: salvar nova', '/unidades/salvar', un());
  await seguir('unidades: lista após criar', '/unidades');
  await post('unidades: sigla repetida (minúscula)', '/unidades/salvar', un({ undSigla: 'cx2' }));
  await seguir('unidades: volta ao formulário novo com erro', '/unidades/novo');
  await post('unidades: sigla vazia', '/unidades/salvar', un({ undSigla: '   ' }));
  await seguir('unidades: …mensagem', '/unidades/novo');
  await post('unidades: sigla > 10', '/unidades/salvar', un({ undSigla: 'ABCDEFGHIJK' }));
  await seguir('unidades: …mensagem', '/unidades/novo');
  await post('unidades: sigla com espaço', '/unidades/salvar', un({ undSigla: 'A B' }));
  await seguir('unidades: …mensagem', '/unidades/novo');
  await post('unidades: descrição vazia', '/unidades/salvar', un({ undSigla: 'ZZ', undDescricao: ' ' }));
  await seguir('unidades: …mensagem', '/unidades/novo');
  await post('unidades: descrição > 60', '/unidades/salvar', un({ undSigla: 'ZZ', undDescricao: 'x'.repeat(61) }));
  await seguir('unidades: …mensagem', '/unidades/novo');
  await post('unidades: nova inativa (sem a caixa marcada)', '/unidades/salvar', { undCodigo: '', undSigla: 'ina', undDescricao: 'Inativa teste', _undAtivo: 'on' });
  await seguir('unidades: lista com inativa', '/unidades');
  await seguir('unidades: editar #6', '/unidades/editar/6');
  await post('unidades: editar trocando sigla (sem uso)', '/unidades/salvar', un({ undCodigo: '6', undSigla: 'CX3', undDescricao: 'Caixa três' }));
  await seguir('unidades: lista', '/unidades');
  await post('unidades: sigla em uma existente de outra', '/unidades/salvar', un({ undCodigo: '6', undSigla: 'un' }));
  await seguir('unidades: …volta à edição', '/unidades/editar/6');
  await post('unidades: editar inexistente (salvar)', '/unidades/salvar', un({ undCodigo: '999', undSigla: 'QQ' }));
  await seguir('unidades: …', '/unidades/editar/999');
  await seguir('unidades: …', '/unidades');
  await post('unidades: inativar #6', '/unidades/status/6', { ativo: 'false' });
  await seguir('unidades: lista', '/unidades');
  await post('unidades: ativar #6', '/unidades/status/6', { ativo: 'true' });
  await post('unidades: status de inexistente', '/unidades/status/999', { ativo: 'true' });
  await seguir('unidades: lista', '/unidades');
  await post('unidades: status sem o parâmetro', '/unidades/status/6', {}, { tela: false });
  await post('unidades: status com valor ruim', '/unidades/status/6', { ativo: 'talvez' }, { tela: false });
  await seguir('unidades: editar id não numérico', '/unidades/editar/abc');
  await post('unidades: excluir #6', '/unidades/excluir/6', {});
  await seguir('unidades: lista', '/unidades');
  await post('unidades: excluir inexistente', '/unidades/excluir/999', {});
  await seguir('unidades: …', '/unidades');

  // unidade em uso por produtos (UN usada por 8 produtos; CJ por 1)
  await seguir('unidades: editar #1 (em uso: sigla travada)', '/unidades/editar/1');
  await post('unidades: trocar a sigla de unidade em uso', '/unidades/salvar', un({ undCodigo: '1', undSigla: 'UNI', undDescricao: 'Unidade' }));
  await seguir('unidades: …volta à edição', '/unidades/editar/1');
  await post('unidades: mudar só a descrição de unidade em uso', '/unidades/salvar', un({ undCodigo: '1', undSigla: 'un', undDescricao: 'Unidade simples' }));
  await seguir('unidades: lista com contagem de produtos', '/unidades');
  await post('unidades: excluir unidade em uso', '/unidades/excluir/2', {});
  await seguir('unidades: …', '/unidades');
  // ───────── Pessoas ─────────
  const pe = (extra = {}) => ({
    upsCodigo: '', upsNome: 'João da Silva', upsCpf: '123.456.789-00', upsTelefone: '(17) 99999-0001', upsNascimento: '1990-05-10',
    upsCliente: 'true', _upsCliente: 'on', _upsFornecedor: 'on', _upsVendedor: 'on', _upsFuncionario: 'on', upsAtivo: 'true', _upsAtivo: 'on',
    segmentoIds: ['1'], ...extra,
  });
  await seguir('pessoas: lista vazia', '/pessoas');
  await seguir('pessoas: formulário novo', '/pessoas/nova');
  await post('pessoas: salvar nova', '/pessoas/salvar', pe());
  await seguir('pessoas: lista', '/pessoas');
  await post('pessoas: CPF repetido com outra máscara', '/pessoas/salvar', pe({ upsNome: 'Outra', upsCpf: '12345678900' }));
  await seguir('pessoas: …', '/pessoas');
  await post('pessoas: telefone repetido só avisa (salva)', '/pessoas/salvar', pe({ upsNome: 'Maria Souza', upsCpf: '987.654.321-00', upsFornecedor: 'true', segmentoIds: ['1', '2'] }));
  await seguir('pessoas: lista', '/pessoas');
  await post('pessoas: nome em branco', '/pessoas/salvar', pe({ upsNome: '  ', upsCpf: '' }));
  await post('pessoas: nome > 150', '/pessoas/salvar', pe({ upsNome: 'N'.repeat(151), upsCpf: '' }));
  await post('pessoas: nascimento futuro', '/pessoas/salvar', pe({ upsNome: 'Futuro', upsCpf: '', upsNascimento: '2099-01-01' }));
  await post('pessoas: CPF com mais de 20 caracteres', '/pessoas/salvar', pe({ upsNome: 'CPF longo', upsCpf: '1'.repeat(21) }));
  await post('pessoas: segmento inexistente', '/pessoas/salvar', pe({ upsNome: 'Seg ruim', upsCpf: '', segmentoIds: ['99'] }), {});
  await seguir('pessoas: …', '/pessoas');
  await seguir('pessoas: editar #1', '/pessoas/editar/1');
  await post('pessoas: editar mantendo o mesmo CPF e trocando segmentos', '/pessoas/salvar', pe({ upsCodigo: '1', upsNome: 'João da Silva Jr', segmentoIds: ['2'], upsVendedor: 'true' }));
  await seguir('pessoas: lista', '/pessoas');
  await post('pessoas: editar inexistente (insere nova)', '/pessoas/salvar', pe({ upsCodigo: '777', upsNome: 'Fantasma', upsCpf: '', upsTelefone: '', upsNascimento: '', segmentoIds: [] }));
  await seguir('pessoas: lista', '/pessoas');
  await seguir('pessoas: editar inexistente', '/pessoas/editar/999');
  await seguir('pessoas: …', '/pessoas');
  await seguir('pessoas: busca com acento e caixa ("JOAO")', '/pessoas?busca=JOAO');
  await seguir('pessoas: busca por CPF parcial', '/pessoas?busca=654.321');
  await seguir('pessoas: busca por telefone', '/pessoas?busca=99999-0001');
  await seguir('pessoas: busca em branco', '/pessoas?busca=%20%20');
  await seguir('pessoas: tipo fornecedor', '/pessoas?tipo=fornecedor');
  await seguir('pessoas: tipo vendedor', '/pessoas?tipo=vendedor');
  await seguir('pessoas: tipo desconhecido', '/pessoas?tipo=xyz');
  await seguir('pessoas: segmento 2', '/pessoas?segmento=2');
  await seguir('pessoas: segmento + tipo + busca', '/pessoas?segmento=1&tipo=cliente&busca=maria');
  await seguir('pessoas: segmento não numérico', '/pessoas?segmento=abc');
  await post('pessoas: excluir #2', '/pessoas/excluir/2', {});
  await seguir('pessoas: lista', '/pessoas');
  await post('pessoas: excluir inexistente', '/pessoas/excluir/999', {});
  await seguir('pessoas: …', '/pessoas');

  await post('pessoas: excluir pessoa com documento (fornecedor da compra)', '/pessoas/excluir/5', {});
  await seguir('pessoas: …', '/pessoas');
  // sem token CSRF: 403 nos dois
  passos.push({ nome: 'CSRF: POST sem token', r: await c.req('/unidades/excluir/1', { method: 'POST', form: {}, semCsrf: true }), tela: false });
  // coleta o dashboard e a lista de unidades finais
  return passos;
}

async function rodarEm(nome) {
  const { base, senha } = BASES[nome];
  const c = new Cliente(base);
  await entrar(c, senha);
  return roteiro(c);
}

function resumo(p) {
  const loc = (p.r.loc || '').replace(/^https?:\/\/[^/]+/, '');
  const linhas = p.tela && p.r.status === 200 ? normalizar(areaConteudo(p.r.texto)) : [];
  return { status: p.r.status, loc, titulo: p.tela && p.r.status === 200 ? titulo(p.r.texto) : '', linhas };
}

const [a, n] = await Promise.all([rodarEm('antigo'), rodarEm('novo')]);
let divergencias = 0;
for (let i = 0; i < a.length; i++) {
  const x = resumo(a[i]);
  const y = resumo(n[i]);
  const problemas = [];
  if (x.status !== y.status) problemas.push(`status antigo ${x.status} ≠ novo ${y.status}`);
  if (x.loc !== y.loc) problemas.push(`redirecionamento antigo "${x.loc}" ≠ novo "${y.loc}"`);
  if (x.titulo !== y.titulo) problemas.push(`título antigo "${x.titulo}" ≠ novo "${y.titulo}"`);
  const d = primeiraDiferenca(x.linhas, y.linhas);
  if (d) problemas.push(`tela diferente, ${d}`);
  if (problemas.length) {
    divergencias++;
    console.log(`✗ ${a[i].nome}\n   ${problemas.join('\n   ')}`);
  } else {
    console.log(`✓ ${a[i].nome}  [${x.status}${x.loc ? ' → ' + x.loc : ''}]${x.linhas.length ? `  (${x.linhas.length} linhas de tela comparadas)` : ''}`);
  }
}
console.log(`\n${a.length} passos, ${divergencias} divergência(s).`);
process.exit(divergencias ? 1 : 0);
