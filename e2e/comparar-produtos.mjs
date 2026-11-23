// Teste GOLDEN de Produtos: o mesmo roteiro no sistema antigo e no novo (ver lib/golden.mjs). Rode por `bash rodar-golden.sh comparar-produtos.mjs`.
// Parte dos dados de exemplo do antigo: produtos 1 a 8 (UN e CJ), estoque 10 nos produtos 1 e 2, uma compra com os dois.
import { executarGolden } from './lib/golden.mjs';

const prod = (extra = {}) => ({
  uprCodigo: '', uprNome: 'Teste Produto', uprComplemento: 'Complemento de teste', uprUnd: 'UN', uprPreco: '12.5',
  uprAtivo: 'true', _uprAtivo: 'on', _uprServico: 'on', uprComissaoPerc: '', uprDuracaoMin: '',
  segmentoId: '', uprCatVenda: '', uprCatCompra: '', ...extra,
});
const un = (siglas, fatores, precos) => ({ unSiglas: siglas, unFatores: fatores, unPrecos: precos });
const servico = (extra = {}) => prod({ uprNome: 'Corte feminino', uprPreco: '80', uprServico: 'true', uprComissaoPerc: '40', uprDuracaoMin: '45', segmentoId: '2', ...extra });

await executarGolden(async (c, { seguir, post }) => {
  // ───────── lista e busca ─────────
  await seguir('lista inicial', '/produtos');
  await seguir('busca "colar"', '/produtos?busca=colar');
  await seguir('busca com acento sem acento ("solitario")', '/produtos?busca=solitario');
  await seguir('busca maiúscula ("PULSEIRA")', '/produtos?busca=PULSEIRA');
  await seguir('busca sem resultado', '/produtos?busca=zzzz');
  await seguir('busca com % (literal)', '/produtos?busca=%25');
  await seguir('busca com _ (literal)', '/produtos?busca=_');
  await seguir('busca em branco', '/produtos?busca=%20');
  await seguir('só produtos', '/produtos?tipo=produto');
  await seguir('só serviços (vazio)', '/produtos?tipo=servico');
  await seguir('tipo desconhecido', '/produtos?tipo=xyz');
  await seguir('formulário novo', '/produtos/novo');
  await seguir('editar #1', '/produtos/editar/1');
  await seguir('editar #6 (unidade CJ)', '/produtos/editar/6');

  // ───────── cadastro simples ─────────
  await post('salvar novo', '/produtos/salvar', prod());
  await seguir('lista', '/produtos');
  await seguir('editar #9', '/produtos/editar/9');
  await post('editar #9 (nome, preço, segmento, categorias)', '/produtos/salvar', prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado', uprPreco: '99.9', segmentoId: '1', uprCatVenda: '1', uprCatCompra: '2' }));
  await seguir('lista', '/produtos');
  await seguir('editar #9 de novo', '/produtos/editar/9');
  await post('preço com arredondamento (12.345)', '/produtos/salvar', prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado', uprPreco: '12.345' }));
  await seguir('editar #9', '/produtos/editar/9');

  // ───────── validações (voltam ao formulário com o digitado) ─────────
  await post('nome em branco', '/produtos/salvar', prod({ uprNome: '  ', uprComplemento: 'mantém isto' }));
  await post('nome > 150', '/produtos/salvar', prod({ uprNome: 'N'.repeat(151) }));
  await post('unidade em branco', '/produtos/salvar', prod({ uprUnd: '' }));
  await post('preço em branco', '/produtos/salvar', prod({ uprPreco: '' }));
  await post('preço zero', '/produtos/salvar', prod({ uprPreco: '0' }));
  await post('preço 0.009 (abaixo de 0,01)', '/produtos/salvar', prod({ uprPreco: '0.009' }));
  await post('preço negativo', '/produtos/salvar', prod({ uprPreco: '-5' }));
  await post('comissão 150', '/produtos/salvar', servico({ uprComissaoPerc: '150' }));
  await post('comissão negativa', '/produtos/salvar', servico({ uprComissaoPerc: '-1' }));
  await post('duração 0', '/produtos/salvar', servico({ uprDuracaoMin: '0' }));
  await post('duração 2000', '/produtos/salvar', servico({ uprDuracaoMin: '2000' }));
  await post('complemento > 255 (sem mensagem na tela)', '/produtos/salvar', prod({ uprComplemento: 'c'.repeat(256) }));
  await post('produto (não serviço) com comissão fora da faixa (valida antes de zerar)', '/produtos/salvar', prod({ uprComissaoPerc: '150' }));
  await post('segmento inexistente', '/produtos/salvar', prod({ segmentoId: '99' }));
  await post('unidade não cadastrada', '/produtos/salvar', prod({ uprUnd: 'ZZ' }));
  await post('unidade com outra caixa ("un")', '/produtos/salvar', prod({ uprNome: 'Unidade minúscula', uprUnd: 'un' }));
  await seguir('lista (a unidade fica como no cadastro)', '/produtos');

  // ───────── unidade inativa ─────────
  await post('inativa a unidade MT', '/unidades/status/5', { ativo: 'false' });
  await post('produto novo em unidade inativa', '/produtos/salvar', prod({ uprNome: 'Em MT', uprUnd: 'MT' }));
  await post('produto existente passa a usar unidade inativa', '/produtos/salvar', prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado', uprUnd: 'MT' }));
  await post('reativa a unidade MT', '/unidades/status/5', { ativo: 'true' });
  await post('produto #8 vai para MT', '/produtos/salvar', prod({ uprCodigo: '8', uprNome: 'Bracelete Rígido Liso', uprPreco: '99.9', uprUnd: 'MT' }));
  await post('inativa a unidade MT (em uso por #8)', '/unidades/status/5', { ativo: 'false' });
  await seguir('editar #8 (unidade inativa continua oferecida)', '/produtos/editar/8');
  await post('salvar #8 mantendo a unidade inativa', '/produtos/salvar', prod({ uprCodigo: '8', uprNome: 'Bracelete Rígido Liso', uprPreco: '99.9', uprUnd: 'MT' }));
  await post('volta #8 para UN', '/produtos/salvar', prod({ uprCodigo: '8', uprNome: 'Bracelete Rígido Liso', uprPreco: '99.9', uprUnd: 'UN' }));
  await post('reativa MT', '/unidades/status/5', { ativo: 'true' });

  // ───────── serviço ─────────
  await post('serviço novo', '/produtos/salvar', servico());
  await seguir('lista com serviço', '/produtos');
  await seguir('só serviços', '/produtos?tipo=servico');
  await seguir('só produtos', '/produtos?tipo=produto');
  await seguir('editar o serviço (#10)', '/produtos/editar/10');
  await post('serviço sem comissão e sem duração', '/produtos/salvar', servico({ uprCodigo: '10', uprComissaoPerc: '', uprDuracaoMin: '' }));
  await seguir('lista', '/produtos');
  await post('serviço com comissão 12.5 e 90 min', '/produtos/salvar', servico({ uprCodigo: '10', uprComissaoPerc: '12.5', uprDuracaoMin: '90' }));
  await seguir('lista', '/produtos');
  await post('produto com estoque vira serviço (#1 tem 10)', '/produtos/salvar', servico({ uprCodigo: '1', uprNome: 'Colar Dourado Delicado' }));
  await post('produto sem estoque vira serviço (#9)', '/produtos/salvar', servico({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }));
  await seguir('lista', '/produtos');
  await post('serviço volta a produto (zera comissão e duração)', '/produtos/salvar', prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado', uprComissaoPerc: '30', uprDuracaoMin: '20' }));
  await seguir('editar #9', '/produtos/editar/9');

  // ───────── unidades extras ─────────
  await post('duas unidades extras', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['CJ', 'PR'], ['12', '2.5'], ['100', '']) });
  await seguir('editar #9 (linhas das unidades)', '/produtos/editar/9');
  await post('mesma sigla repetida', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['CJ', 'cj'], ['12', '3'], ['', '']) });
  await post('extra igual à base', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['un'], ['2'], ['']) });
  await post('fator zero', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['CJ'], ['0'], ['']) });
  await post('fator em branco', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['CJ'], [''], ['']) });
  await post('fator acima de 1.000.000', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['CJ'], ['1000001'], ['']) });
  await post('preço da unidade zero', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['CJ'], ['12'], ['0']) });
  await post('linha sem unidade', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un([''], ['12'], ['']) });
  await post('unidade extra não cadastrada', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['ZZ'], ['12'], ['']) });
  await post('linha totalmente em branco é ignorada', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un([''], [''], ['']) });
  await seguir('editar #9 (extras apagadas)', '/produtos/editar/9');
  await post('três extras de novo', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado' }), ...un(['KG', 'CJ', 'PR'], ['0.5', '12', '2'], ['7.5', '', '']) });
  await post('trocar a base com extras no formulário', '/produtos/salvar', { ...prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado', uprUnd: 'MT' }), ...un(['CJ'], ['12'], ['']) });
  await seguir('editar #9', '/produtos/editar/9');
  await post('trocar a base removendo as extras', '/produtos/salvar', prod({ uprCodigo: '9', uprNome: 'Teste Produto Editado', uprUnd: 'MT' }));
  await seguir('editar #9', '/produtos/editar/9');
  await seguir('unidades: contagem de produtos por unidade', '/unidades');

  // ───────── status ─────────
  await post('inativar #9', '/produtos/status/9', { ativo: 'false' });
  await seguir('lista com inativo', '/produtos');
  await seguir('busca não traz inativo ("teste")', '/produtos?busca=teste');
  await post('ativar #9', '/produtos/status/9', { ativo: 'true' });
  await post('status de inexistente', '/produtos/status/999', { ativo: 'true' });
  await post('status sem parâmetro', '/produtos/status/9', {}, { tela: false });
  await post('status com valor ruim', '/produtos/status/9', { ativo: 'talvez' }, { tela: false });
  await seguir('lista', '/produtos');
  await seguir('editar inexistente', '/produtos/editar/999');
  await seguir('editar id não numérico', '/produtos/editar/abc', { tela: false });

  // ───────── falhas depois de gravar (no SQL Server consomem o código; por isso ficam por último, antes dos novos cadastros) ─────────
  await post('novo produto com unidade extra inválida (nada é gravado)', '/produtos/salvar', { ...prod({ uprNome: 'Não deve existir' }), ...un(['ZZ'], ['2'], ['']) });
  await seguir('lista (o produto não existe)', '/produtos');

  // ───────── excluir ─────────
  await post('excluir produto com documento (#1)', '/produtos/excluir/1', {});
  await seguir('lista', '/produtos');
  await post('excluir produto sem documento (#9): o antigo sempre falha (D-03); o novo exclui', '/produtos/excluir/9', {});
  await seguir('lista', '/produtos', { esperaDivergir: 'D-03' });
  await post('excluir inexistente', '/produtos/excluir/999', {});
  await seguir('lista', '/produtos', { esperaDivergir: 'D-03' });

  // sem token CSRF: 403 nos dois
  await post('CSRF: POST sem token', '/produtos/excluir/1', null, { tela: false });
});
