// Teste GOLDEN de Usuários e Permissões: o mesmo roteiro no sistema antigo e no novo (ver lib/golden.mjs).
// Rode por `bash rodar-golden.sh comparar-usuarios.mjs`. Parte dos dados de exemplo do antigo: só o admin (#1); Carlos (#7) e
// Fernanda (#8) são vendedores ativos; Maria Clara (#1 em Pessoas) não é vendedora.
import { executarGolden } from './lib/golden.mjs';

const usu = (extra = {}) => ({
  usuCodigo: '', usuNome: 'Maria Souza', usuLogin: 'maria', novaSenha: 'Senha-123', usuAtivo: 'true', _usuAtivo: 'on', vendedorId: '', perfil: '', ...extra,
});
const mod = (nome, acoes) => acoes.split('').map((a) => ({ C: 'consulta_', I: 'inclusao_', A: 'atualizacao_', E: 'exclusao_' })[a] + nome);
const marcar = (...campos) => Object.fromEntries(campos.flat().map((c) => [c, 'on']));

await executarGolden(async (c, { seguir, post }) => {
  // ───────── lista e formulários ─────────
  await seguir('lista inicial (só o admin)', '/usuarios');
  await seguir('formulário novo', '/usuarios/novo');
  await seguir('editar o admin (#1)', '/usuarios/editar/1');

  // ───────── cadastro ─────────
  await post('novo sem perfil', '/usuarios/salvar', usu());
  await seguir('lista', '/usuarios');
  await post('login repetido (outra caixa)', '/usuarios/salvar', usu({ usuNome: 'Outra', usuLogin: 'MARIA' }));
  await seguir('lista', '/usuarios');
  await post('login repetido com acento ("María")', '/usuarios/salvar', usu({ usuNome: 'Outra', usuLogin: 'María' }));
  await seguir('lista', '/usuarios');
  await post('login com espaços nas pontas é aparado', '/usuarios/salvar', usu({ usuNome: 'Joana Lima', usuLogin: '  joana  ' }));
  await seguir('lista', '/usuarios');
  await post('senha em branco para usuário novo', '/usuarios/salvar', usu({ usuNome: 'Sem senha', usuLogin: 'semsenha', novaSenha: '   ' }));
  await post('nome em branco (volta sem mensagem)', '/usuarios/salvar', usu({ usuNome: ' ', usuLogin: 'x1' }));
  await post('login em branco', '/usuarios/salvar', usu({ usuLogin: '' }));
  await post('login com mais de 80 caracteres', '/usuarios/salvar', usu({ usuLogin: 'l'.repeat(81) }));
  await post('nome com mais de 150 caracteres', '/usuarios/salvar', usu({ usuNome: 'N'.repeat(151), usuLogin: 'x2' }));
  await post('vendedor inexistente', '/usuarios/salvar', usu({ usuNome: 'V1', usuLogin: 'v1', vendedorId: '99' }));
  await seguir('lista', '/usuarios');
  await post('vendedor que não é vendedor (pessoa #1)', '/usuarios/salvar', usu({ usuNome: 'V2', usuLogin: 'v2', vendedorId: '1' }));
  await seguir('lista', '/usuarios');
  await post('novo com vendedor e perfil Vendedor da loja', '/usuarios/salvar', usu({ usuNome: 'Carlos Vendedor', usuLogin: 'carlos', vendedorId: '7', perfil: 'VENDEDOR_LOJA' }));
  await seguir('lista com vendedor', '/usuarios');
  await post('novo com perfil Gerência', '/usuarios/salvar', usu({ usuNome: 'Gerente', usuLogin: 'gerente', perfil: 'GERENCIA' }));
  await seguir('lista', '/usuarios');
  await post('novo inativo (sem a caixa)', '/usuarios/salvar', { usuCodigo: '', usuNome: 'Inativo', usuLogin: 'inativo', novaSenha: 'Senha-123', _usuAtivo: 'on', vendedorId: '', perfil: '' });
  await seguir('lista', '/usuarios');

  // ───────── editar ─────────
  await seguir('editar #2', '/usuarios/editar/2');
  await post('editar #2: nome, login, vendedor e senha em branco', '/usuarios/salvar', usu({ usuCodigo: '2', usuNome: 'Maria Souza Lima', usuLogin: 'maria.lima', novaSenha: '', vendedorId: '8' }));
  await seguir('lista', '/usuarios');
  await post('editar #2 com senha nova', '/usuarios/salvar', usu({ usuCodigo: '2', usuNome: 'Maria Souza Lima', usuLogin: 'maria.lima', novaSenha: 'Outra-Senha-9', vendedorId: '8' }));
  await post('editar #2 para login de outro usuário', '/usuarios/salvar', usu({ usuCodigo: '2', usuNome: 'Maria', usuLogin: 'carlos', novaSenha: '' }));
  await post('editar #2 tirando o vendedor e inativando', '/usuarios/salvar', usu({ usuCodigo: '2', usuNome: 'Maria Souza Lima', usuLogin: 'maria.lima', novaSenha: '', vendedorId: '', usuAtivo: '', _usuAtivo: 'on' }));
  await seguir('lista', '/usuarios');
  await post('editar usuário inexistente', '/usuarios/salvar', usu({ usuCodigo: '999', usuLogin: 'fantasma', novaSenha: '' }));
  await seguir('editar inexistente', '/usuarios/editar/999');
  await seguir('…', '/usuarios');
  await seguir('editar id não numérico', '/usuarios/editar/abc', { tela: false });
  await post('admin: mudar o login', '/usuarios/salvar', usu({ usuCodigo: '1', usuNome: 'Administrador', usuLogin: 'chefe', novaSenha: '' }));
  await post('admin: login com outra caixa é permitido', '/usuarios/salvar', usu({ usuCodigo: '1', usuNome: 'Administrador', usuLogin: 'ADMIN', novaSenha: '' }));
  await seguir('lista', '/usuarios');
  await post('admin: inativar pelo formulário', '/usuarios/salvar', usu({ usuCodigo: '1', usuNome: 'Administrador', usuLogin: 'admin', novaSenha: '', usuAtivo: '', _usuAtivo: 'on' }));
  await post('admin: voltar o login para minúsculas', '/usuarios/salvar', usu({ usuCodigo: '1', usuNome: 'Administrador', usuLogin: 'admin', novaSenha: '' }));
  await seguir('lista', '/usuarios');

  // ───────── status e exclusão ─────────
  await post('inativar #3', '/usuarios/status/3', { ativo: 'false' });
  await seguir('lista', '/usuarios');
  await post('ativar #3', '/usuarios/status/3', { ativo: 'true' });
  await post('inativar o admin', '/usuarios/status/1', { ativo: 'false' });
  await seguir('lista', '/usuarios');
  await post('ativar o admin (não muda nada)', '/usuarios/status/1', { ativo: 'true' });
  await post('status de inexistente', '/usuarios/status/999', { ativo: 'true' });
  await seguir('lista', '/usuarios');
  await post('status sem parâmetro', '/usuarios/status/3', {}, { tela: false });
  await post('status com valor ruim', '/usuarios/status/3', { ativo: 'talvez' }, { tela: false });
  await post('excluir o admin', '/usuarios/excluir/1', {});
  await seguir('lista', '/usuarios');
  await post('excluir #5 (inativo, sem documentos)', '/usuarios/excluir/5', {});
  await seguir('lista', '/usuarios');
  await post('excluir inexistente', '/usuarios/excluir/999', {});
  await seguir('lista', '/usuarios');

  // ───────── permissões ─────────
  await seguir('permissões do admin (#1)', '/usuarios/permissoes/1');
  await seguir('permissões do #2 (padrão de usuário novo)', '/usuarios/permissoes/2');
  await seguir('permissões do #3 (perfil Vendedor da loja)', '/usuarios/permissoes/3');
  await seguir('permissões do #4 (perfil Gerência)', '/usuarios/permissoes/4');
  await seguir('permissões de inexistente', '/usuarios/permissoes/999');
  await seguir('…', '/usuarios');
  await seguir('permissões com id não numérico', '/usuarios/permissoes/abc', { tela: false });
  await post('salvar matriz do #2', '/usuarios/permissoes/2', marcar(mod('PRODUTOS', 'CIAE'), mod('PESSOAS', 'CI'), mod('METAS', 'CI'), mod('VENDAS_TODOS', 'CIAE'), mod('COMISSOES', 'C'), mod('BI_EXECUTIVO', 'C'), mod('FIN_DRE', 'C')));
  await seguir('permissões do #2 depois de salvar', '/usuarios/permissoes/2');
  await post('salvar matriz vazia do #2', '/usuarios/permissoes/2', {});
  await seguir('permissões do #2 zeradas', '/usuarios/permissoes/2');
  await post('aplicar perfil Recepção no #2', '/usuarios/permissoes/2/perfil', { perfil: 'RECEPCAO' });
  await seguir('permissões do #2 com Recepção', '/usuarios/permissoes/2');
  await post('aplicar perfil Profissional do salão no #2', '/usuarios/permissoes/2/perfil', { perfil: 'PROFISSIONAL_SALAO' });
  await seguir('permissões do #2 com Profissional', '/usuarios/permissoes/2');
  await post('aplicar perfil Gerência no #2', '/usuarios/permissoes/2/perfil', { perfil: 'GERENCIA' });
  await seguir('permissões do #2 com Gerência', '/usuarios/permissoes/2');
  await post('aplicar perfil Vendedor da loja no #2', '/usuarios/permissoes/2/perfil', { perfil: 'VENDEDOR_LOJA' });
  await seguir('permissões do #2 com Vendedor', '/usuarios/permissoes/2');
  await post('perfil inexistente', '/usuarios/permissoes/2/perfil', { perfil: 'NADA' });
  await seguir('…', '/usuarios/permissoes/2');
  await post('perfil em usuário inexistente', '/usuarios/permissoes/999/perfil', { perfil: 'RECEPCAO' });
  await seguir('…', '/usuarios/permissoes/999');
  await post('perfil no admin', '/usuarios/permissoes/1/perfil', { perfil: 'RECEPCAO' });
  await seguir('permissões do admin', '/usuarios/permissoes/1');
  await post('perfil sem o parâmetro', '/usuarios/permissoes/2/perfil', {}, { tela: false });
  await post('salvar matriz de usuário inexistente', '/usuarios/permissoes/999', marcar(mod('PRODUTOS', 'C')));
  await seguir('…', '/usuarios/permissoes/999');
  await post('salvar matriz do admin (forjado: o botão fica desabilitado)', '/usuarios/permissoes/1', marcar(mod('PRODUTOS', 'C')));
  await seguir('permissões do admin continuam completas', '/usuarios/permissoes/1');

  // ───────── falha depois de gravar (queima o código) ─────────
  await post('novo usuário com perfil inexistente (desfaz tudo)', '/usuarios/salvar', usu({ usuNome: 'Falha', usuLogin: 'falha', perfil: 'NADA' }));
  await seguir('lista (não existe)', '/usuarios');
  await post('próximo usuário usa o código seguinte ao queimado', '/usuarios/salvar', usu({ usuNome: 'Depois', usuLogin: 'depois' }));
  await seguir('lista', '/usuarios');

  await post('CSRF: POST sem token', '/usuarios/excluir/1', null, { tela: false });
});
