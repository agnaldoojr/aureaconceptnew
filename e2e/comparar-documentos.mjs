// Teste GOLDEN de Documentos e Estoque: o mesmo roteiro no sistema antigo e no novo (ver lib/golden.mjs), mais a comparação do ESTADO FINAL
// do banco (estoque, documentos, itens, títulos, baixas e movimentos; ver despejo.mjs). Rode por `bash rodar-golden.sh comparar-documentos.mjs`.
// Dados de exemplo do antigo: produtos 1 a 8 (UN; o #6 em CJ), estoque 10 un. do #1 (custo 35) e do #2 (custo 15), compra #1; pessoas 1-4 clientes,
// 5-6 fornecedores, 7-8 vendedores; formas: 1 Dinheiro*, 2 PIX*, 3 Débito, 4 Crédito (12x), 5 Boleto (6x), 6 Crediário, 7 Dinheiro pag.*, 8 PIX pag.*,
// 9 Boleto a pagar (6x) (*baixa automática).
import { executarGolden } from './lib/golden.mjs';

const prod = (extra = {}) => ({
  uprCodigo: '', uprNome: 'Teste', uprComplemento: '', uprUnd: 'UN', uprPreco: '10', uprAtivo: 'true', _uprAtivo: 'on', _uprServico: 'on',
  uprComissaoPerc: '', uprDuracaoMin: '', segmentoId: '', uprCatVenda: '', uprCatCompra: '', ...extra,
});
const item = (produto, qtd, valor, extra = {}) => ({ produto, qtd, valor, und: '', prof: '', ...extra });
const doc = (cab, itens) => ({
  doctoCodigo: cab.codigo ?? '', doctoTipo: cab.tipo ?? '', doctoData: cab.data ?? '2026-11-15', 'pessoa.upsCodigo': cab.pessoa ?? '1',
  segmentoId: cab.seg ?? '', vendedorId: cab.vend ?? '',
  ...(cab.fpg !== undefined ? { fpgCodigo: cab.fpg, parcelas: cab.parc ?? '1', primeiroVencimento: cab.venc ?? '', catCodigo: cab.cat ?? '' } : {}),
  ...(cab.classif ? { doctoClassificacao: cab.classif } : {}),
  produtoIds: itens.map((i) => i.produto), unidades: itens.map((i) => i.und), quantidades: itens.map((i) => String(i.qtd)),
  valores: itens.map((i) => String(i.valor)), profissionalIds: itens.map((i) => i.prof),
});

await executarGolden(async (c, { seguir, post, postVer }) => {
  const lancar = (nome, cab, itens, opcoes) => postVer(nome, '/documentos/salvar', doc(cab, itens), opcoes);

  // ───────── telas iniciais ─────────
  await seguir('lista inicial', '/documentos');
  await seguir('filtro por tipo COMPRA', '/documentos?tipo=COMPRA&inicio=&fim=');
  await seguir('filtro "Todos os tipos" (tipo vazio)', '/documentos?tipo=&inicio=&fim=');
  await seguir('filtro por datas', '/documentos?inicio=2020-01-01&fim=2099-12-31');
  await seguir('filtro por datas sem resultado', '/documentos?inicio=2000-01-01&fim=2000-12-31');
  await seguir('filtro com data inválida', '/documentos?inicio=abc', { tela: false });
  await seguir('estoque (todos)', '/estoque');
  await seguir('estoque baixo', '/estoque?filtro=baixo');
  await seguir('formulário novo', '/documentos/novo');
  await seguir('visualizar #1', '/documentos/visualizar/1');
  await seguir('editar #1', '/documentos/editar/1');
  await seguir('visualizar inexistente', '/documentos/visualizar/999');
  await seguir('…', '/documentos');
  await seguir('editar inexistente', '/documentos/editar/999');
  await seguir('…', '/documentos');

  // ───────── dados de apoio (pelas telas de produtos) ─────────
  await post('produto 9: Caixa (CX = 12 UN a R$ 100; DZ = 12 sem preço)', '/produtos/salvar', {
    ...prod({ uprNome: 'Caixa teste', uprPreco: '10', uprCatCompra: '7' }),
    unSiglas: ['CJ', 'MT'], unFatores: ['12', '0.5'], unPrecos: ['100', ''],
  });
  await post('produto 10: serviço com comissão 40% (segmento Salão)', '/produtos/salvar', prod({ uprNome: 'Corte', uprPreco: '80', uprServico: 'true', uprComissaoPerc: '40', uprDuracaoMin: '45', segmentoId: '2', uprCatVenda: '2' }));
  await post('produto 11: serviço sem comissão', '/produtos/salvar', prod({ uprNome: 'Hidratação', uprPreco: '50', uprServico: 'true', segmentoId: '2' }));
  await post('produto 12: preço quebrado', '/produtos/salvar', prod({ uprNome: 'Fita', uprPreco: '9.99' }));
  await seguir('estoque com os novos', '/estoque');

  // ───────── ENTRADA e SAÍDA ─────────
  await lancar('entrada (ajuste) de 5 un. do #3 a R$ 20', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('3', 5, 20)]);
  await seguir('estoque', '/estoque');
  await lancar('saída (ajuste) de 2 un. do #3', { tipo: 'SAIDA', classif: 'AJUSTE' }, [item('3', 2, 20)]);
  await lancar('saída (quebra) de 1 un. do #3', { tipo: 'SAIDA', classif: 'QUEBRA' }, [item('3', 1, 20)]);
  await lancar('entrada (transferência) com minúscula na classificação', { tipo: 'ENTRADA', classif: 'transferencia' }, [item('3', 1, 20)]);
  await lancar('entrada marcada como quebra', { tipo: 'ENTRADA', classif: 'QUEBRA' }, [item('3', 1, 20)]);
  await lancar('entrada sem classificação', { tipo: 'ENTRADA' }, [item('3', 1, 20)]);
  await lancar('saída sem classificação', { tipo: 'SAIDA' }, [item('3', 1, 20)]);
  await lancar('classificação inválida', { tipo: 'SAIDA', classif: 'FURTO' }, [item('3', 1, 20)]);
  await lancar('saída maior que o saldo', { tipo: 'SAIDA', classif: 'AJUSTE' }, [item('3', 10, 20)]);
  await lancar('saída com dois itens que juntos passam do saldo (falha depois de gravar: queima códigos)', { tipo: 'SAIDA', classif: 'AJUSTE' }, [item('3', 2, 20), item('3', 2, 20)]);
  await lancar('serviço em entrada', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('10', 1, 80)]);
  await lancar('serviço em saída', { tipo: 'SAIDA', classif: 'AJUSTE' }, [item('11', 1, 50)]);
  await lancar('documento sem itens', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('', 1, 20)]);
  await lancar('entrada na unidade CJ (fator 12): 2 CJ a R$ 100', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('9', 2, 100, { und: 'CJ' })]);
  await lancar('entrada na unidade MT (fator 0,5): 3 MT a R$ 7', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('9', 3, 7, { und: 'mt' })]);
  await lancar('unidade que o produto não tem', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('9', 1, 7, { und: 'KG' })]);
  await lancar('custo médio ponderado: 5 un. do #1 a R$ 40 (já tinha 10 a 35)', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('1', 5, 40)]);
  await lancar('quantidade fracionada e preço quebrado: 0,125 de #12 a 9,99', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('12', 0.125, 9.99)]);
  await lancar('mais 3,333 de #12 a 9,99 (custo médio com arredondamento)', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('12', 3.333, 9.99)]);
  await lancar('quantidade com mais de 3 casas (1,2345) a 3,33', { tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('12', 1.2345, 3.33)]);
  await seguir('estoque', '/estoque');
  await seguir('estoque baixo', '/estoque?filtro=baixo');
  await seguir('lista de documentos', '/documentos');
  await seguir('lista só entradas', '/documentos?tipo=ENTRADA');

  // ───────── COMPRA (a pagar) ─────────
  await lancar('compra com PIX pagar (baixa automática)', { tipo: 'COMPRA', pessoa: '5', fpg: '8', data: '2026-11-10' }, [item('2', 4, 16.5), item('3', 1, 21)]);
  await lancar('compra boleto 3x com 1º vencimento informado', { tipo: 'COMPRA', pessoa: '6', fpg: '9', parc: '3', venc: '2026-12-05', data: '2026-11-11' }, [item('4', 7, 10.01)]);
  await lancar('compra boleto 4x sem vencimento (usa os dias da forma)', { tipo: 'COMPRA', pessoa: '6', fpg: '9', parc: '4', data: '2026-11-12' }, [item('5', 3, 33.33)]);
  await lancar('compra com categoria escolhida (Aluguel)', { tipo: 'COMPRA', pessoa: '5', fpg: '9', cat: '6', data: '2026-11-13' }, [item('6', 1, 100)]);
  await lancar('compra sem categoria: usa a do cadastro do produto (#9 → Água, luz e internet)', { tipo: 'COMPRA', pessoa: '5', fpg: '9', data: '2026-11-13' }, [item('9', 1, 10)]);
  await lancar('compra com parcelas em forma que não parcela (Dinheiro pagar, 3 → 1)', { tipo: 'COMPRA', pessoa: '5', fpg: '7', parc: '3', data: '2026-11-14' }, [item('7', 2, 5)]);
  await lancar('compra sem forma de pagamento', { tipo: 'COMPRA', pessoa: '5' }, [item('7', 1, 5)]);
  await lancar('compra com forma de recebimento (Dinheiro receber)', { tipo: 'COMPRA', pessoa: '5', fpg: '1' }, [item('7', 1, 5)]);
  await lancar('compra com parcelas acima do máximo (7 de 6)', { tipo: 'COMPRA', pessoa: '5', fpg: '9', parc: '7' }, [item('7', 1, 5)]);
  await lancar('compra com forma inexistente', { tipo: 'COMPRA', pessoa: '5', fpg: '99' }, [item('7', 1, 5)]);
  await lancar('compra com categoria inexistente', { tipo: 'COMPRA', pessoa: '5', fpg: '9', cat: '99' }, [item('7', 1, 5)]);
  await lancar('compra de serviço (permitida, sem estoque)', { tipo: 'COMPRA', pessoa: '5', fpg: '9' }, [item('11', 1, 50)]);

  // ───────── VENDA (a receber) ─────────
  await lancar('venda em Dinheiro (baixa automática)', { tipo: 'VENDA', pessoa: '1', fpg: '1', vend: '7', data: '2026-11-15' }, [item('1', 2, 89.9), item('2', 1, 49.9)]);
  await lancar('venda em PIX', { tipo: 'VENDA', pessoa: '2', fpg: '2', data: '2026-11-15' }, [item('4', 1, 79.9)]);
  await lancar('venda no débito (1 parcela, vence em 1 dia)', { tipo: 'VENDA', pessoa: '3', fpg: '3', data: '2026-11-15' }, [item('1', 1, 89.9)]);
  await lancar('venda no crédito 3x (taxa só na baixa)', { tipo: 'VENDA', pessoa: '3', fpg: '4', parc: '3', data: '2026-11-15' }, [item('1', 1, 100), item('2', 1, 49.9)]);
  await lancar('venda no crediário 5x com total que não divide (1 de 5 x 33,33...)', { tipo: 'VENDA', pessoa: '4', fpg: '6', parc: '5', venc: '2026-12-01', data: '2026-11-15' }, [item('3', 1, 100.01)]);
  await lancar('venda com forma de pagamento (pagar)', { tipo: 'VENDA', pessoa: '1', fpg: '8' }, [item('1', 1, 10)]);
  await lancar('venda com mais do que o estoque', { tipo: 'VENDA', pessoa: '1', fpg: '1' }, [item('2', 50, 49.9)]);
  await lancar('venda na unidade CJ do produto 9 (12 un. por CJ)', { tipo: 'VENDA', pessoa: '1', fpg: '1', data: '2026-11-15' }, [item('9', 1, 100, { und: 'CJ' })]);
  await lancar('venda com valor quebrado em boleto (parcela de total com mais de 2 casas)', { tipo: 'VENDA', pessoa: '1', fpg: '5', parc: '2', data: '2026-11-15' }, [item('12', 0.5, 9.99)]);
  await lancar('venda de serviço com comissão e profissional', { tipo: 'VENDA', pessoa: '2', fpg: '2', vend: '7', data: '2026-11-15' }, [item('10', 1, 80, { prof: '7' })]);
  await lancar('venda de serviço com comissão sem profissional', { tipo: 'VENDA', pessoa: '2', fpg: '2' }, [item('10', 1, 80)]);
  await lancar('venda de serviço com profissional que não é vendedor', { tipo: 'VENDA', pessoa: '2', fpg: '2' }, [item('10', 1, 80, { prof: '1' })]);
  await lancar('venda de serviço sem comissão (profissional ignorado)', { tipo: 'VENDA', pessoa: '2', fpg: '2', data: '2026-11-15' }, [item('11', 1, 50, { prof: '8' })]);
  await lancar('venda de produto com profissional (ignorado)', { tipo: 'VENDA', pessoa: '2', fpg: '2', data: '2026-11-15' }, [item('4', 1, 79.9, { prof: '8' })]);
  await lancar('venda com vendedor que não é vendedor', { tipo: 'VENDA', pessoa: '2', fpg: '2', vend: '1' }, [item('4', 1, 79.9)]);
  await lancar('venda misturando produto (Loja) e serviço (Salão): segmento do item de maior valor', { tipo: 'VENDA', pessoa: '2', fpg: '2', vend: '8', data: '2026-11-15' }, [item('4', 1, 79.9), item('10', 2, 80, { prof: '8' })]);
  await lancar('venda com segmento escolhido (Loja de roupa)', { tipo: 'VENDA', pessoa: '2', fpg: '2', seg: '1', data: '2026-11-15' }, [item('10', 1, 80, { prof: '7' })]);
  await lancar('venda com segmento inexistente', { tipo: 'VENDA', pessoa: '2', fpg: '2', seg: '99' }, [item('4', 1, 79.9)]);
  await seguir('estoque depois das vendas', '/estoque');
  await seguir('lista de documentos', '/documentos');
  await seguir('só vendas', '/documentos?tipo=VENDA');
  await seguir('visualizar a venda do serviço', '/documentos/visualizar/37');
  await seguir('visualizar a compra com unidade CJ', '/documentos/visualizar/16');
  await seguir('visualizar a venda em CJ', '/documentos/visualizar/31');

  // ───────── editar ─────────
  await seguir('editar a venda em Dinheiro (#26)', '/documentos/editar/26');
  await postVer('editar #26: quantidade e valor', '/documentos/salvar', doc({ codigo: '26', tipo: 'VENDA', pessoa: '1', fpg: '1', vend: '7', data: '2026-11-15' }, [item('1', 3, 89.9)]));
  await seguir('editar #26 de novo', '/documentos/editar/26');
  await postVer('editar #26 trocando para PIX e outro cliente', '/documentos/salvar', doc({ codigo: '26', tipo: 'VENDA', pessoa: '2', fpg: '2', data: '2026-11-14' }, [item('1', 3, 89.9), item('4', 1, 79.9)]));
  await postVer('editar #26 passando de venda para saída', '/documentos/salvar', doc({ codigo: '26', tipo: 'SAIDA', classif: 'AJUSTE', data: '2026-11-14' }, [item('1', 1, 89.9)]));
  await postVer('editar #26 voltando para venda parcelada', '/documentos/salvar', doc({ codigo: '26', tipo: 'VENDA', pessoa: '2', fpg: '5', parc: '3', venc: '2026-12-10', data: '2026-11-14' }, [item('1', 1, 89.9)]));
  await seguir('editar #26 com título em aberto', '/documentos/editar/26');
  await postVer('editar com falta de estoque na saída', '/documentos/salvar', doc({ codigo: '26', tipo: 'VENDA', pessoa: '2', fpg: '2' }, [item('2', 99, 1)]));
  await postVer('editar sem itens', '/documentos/salvar', doc({ codigo: '26', tipo: 'VENDA', pessoa: '2', fpg: '2' }, [item('', 1, 1)]));
  await postVer('editar documento inexistente', '/documentos/salvar', doc({ codigo: '999', tipo: 'ENTRADA', classif: 'AJUSTE' }, [item('3', 1, 1)]));
  await postVer('editar a compra #1 (entrada de 10 do #1 e 10 do #2): o estoque do #1 foi consumido?', '/documentos/salvar', doc({ codigo: '1', tipo: 'COMPRA', pessoa: '5', fpg: '9' }, [item('1', 10, 35), item('2', 10, 15)]));
  await seguir('estoque', '/estoque');

  // ───────── excluir ─────────
  await postVer('excluir a venda #26 (reverte o estoque e apaga os títulos)', '/documentos/excluir/26', {});
  await postVer('excluir uma compra cujo estoque já foi consumido (#1)', '/documentos/excluir/1', {});
  await postVer('excluir a compra com PIX (#16 com baixa automática)', '/documentos/excluir/16', {});
  await postVer('excluir documento inexistente', '/documentos/excluir/999', {});
  await seguir('estoque no fim', '/estoque');
  await seguir('lista no fim', '/documentos');


  // ───────── DIFERENÇA DECIDIDA (D-21), por último para não deslocar os códigos dos passos acima ─────────
  // O antigo recusa a venda cujo total tem fração de centavo que arredonda para cima numa forma de baixa automática ("O valor da baixa (R$ 5.00) é
  // maior que o saldo do título (R$ 4.995)"); o novo grava a venda. Depois da comparação, desfaz-se a venda do novo (fora do roteiro registrado) para o
  // estado final do banco continuar comparável.
  const dif = await lancar('venda com total de 4,995 em PIX (baixa automática): o antigo recusa; o novo grava (D-21)', { tipo: 'VENDA', pessoa: '1', fpg: '2', data: '2026-11-15' }, [item('12', 0.5, 9.99)], { esperaDivergir: 'D-21' });
  if (dif.loc && dif.loc.endsWith('/documentos')) { // sucesso (só acontece no novo)
    const lista = await c.req('/documentos');
    const m = lista.texto.match(/<td>(\d+)<\/td>\s*<td>\s*<span class="badge/);
    if (m) await c.req('/documentos/excluir/' + m[1], { method: 'POST', form: {} });
  }

  await post('CSRF: POST sem token', '/documentos/excluir/2', null, { tela: false });
});
