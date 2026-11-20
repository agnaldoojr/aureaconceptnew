import java.io.PrintWriter;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;

/**
 * Gera os valores de REFERÊNCIA das regras de dinheiro, estoque e financeiro executando as MESMAS fórmulas do sistema
 * antigo (Java/BigDecimal), copiadas de EstoqueService, ItemDocumento, FinanceiroService, ComissaoService e MetaVendaService
 * (commit 7c3b487 de agnaldoojr/aureaconcept). O teste Go em internal/dominio compara o resultado dele com este arquivo.
 *
 * Uso (só desenvolvimento; Java não entra na imagem de produção):
 *   java tools/golden/Golden.java testdata/golden/dinheiro.csv
 */
public class Golden {
    static final BigDecimal CEM = new BigDecimal("100");
    static Random rnd = new Random(20261120L);

    static BigDecimal d(long unscaled, int scale) { return BigDecimal.valueOf(unscaled, scale); }

    static BigDecimal aleat(long max, int scale) { return d(1 + (long) (rnd.nextDouble() * max), scale); }

    // ItemDocumento.getQtdBase / getValorBase
    static BigDecimal qtdBase(BigDecimal qtd, BigDecimal fator) {
        BigDecimal f = fator == null || fator.signum() <= 0 ? BigDecimal.ONE : fator;
        return qtd.multiply(f).setScale(3, RoundingMode.HALF_UP);
    }

    static BigDecimal valorBase(BigDecimal valor, BigDecimal fator) {
        BigDecimal f = fator == null || fator.signum() <= 0 ? BigDecimal.ONE : fator;
        return valor.divide(f, 4, RoundingMode.HALF_UP);
    }

    // EstoqueService.entrada
    static BigDecimal custoMedio(BigDecimal qtdAtual, BigDecimal custoAtual, BigDecimal quantidade, BigDecimal custoUnitario) {
        BigDecimal totalAnterior = qtdAtual.multiply(custoAtual);
        BigDecimal totalNovo = quantidade.multiply(custoUnitario);
        BigDecimal novaQtd = qtdAtual.add(quantidade);
        return novaQtd.compareTo(BigDecimal.ZERO) > 0
                ? totalAnterior.add(totalNovo).divide(novaQtd, 4, RoundingMode.HALF_UP)
                : BigDecimal.ZERO;
    }

    public static void main(String[] args) throws Exception {
        String saida = args.length > 0 ? args[0] : "dinheiro.csv";
        try (PrintWriter w = new PrintWriter(saida, StandardCharsets.UTF_8)) {
            w.println("# Gerado por tools/golden/Golden.java (fórmulas do sistema antigo, BigDecimal). NÃO editar à mão.");
            // QB / VB: quantidade e valor na unidade base
            for (int i = 0; i < 120; i++) {
                BigDecimal qtd = aleat(500_000, 3);
                BigDecimal valor = aleat(99_999, 2);
                BigDecimal fator = i % 4 == 0 ? BigDecimal.ONE : aleat(1_000_000_000L, 6);
                w.println("QB;" + qtd.toPlainString() + ";" + fator.toPlainString() + ";" + qtdBase(qtd, fator).toPlainString());
                w.println("VB;" + valor.toPlainString() + ";" + fator.toPlainString() + ";" + valorBase(valor, fator).toPlainString());
            }
            // fator zero/negativo vira 1
            w.println("QB;10.000;0.000000;" + qtdBase(new BigDecimal("10.000"), BigDecimal.ZERO).toPlainString());
            w.println("VB;12.50;-3.000000;" + valorBase(new BigDecimal("12.50"), new BigDecimal("-3")).toPlainString());
            // CM: custo médio ponderado
            for (int i = 0; i < 200; i++) {
                BigDecimal saldo = i % 10 == 0 ? BigDecimal.ZERO : aleat(1_000_000, 3);
                BigDecimal custo = i % 10 == 0 ? BigDecimal.ZERO : aleat(9_999_999, 4);
                BigDecimal qtd = aleat(300_000, 3);
                BigDecimal valor = aleat(9_999_999, 4);
                w.println("CM;" + saldo.toPlainString() + ";" + custo.toPlainString() + ";" + qtd.toPlainString() + ";" + valor.toPlainString()
                        + ";" + custoMedio(saldo, custo, qtd, valor).toPlainString());
            }
            // PARC: parcelas (ROUND_DOWN e resto na última)
            for (int i = 0; i < 150; i++) {
                BigDecimal total = aleat(2_000_000, 2);
                int n = 1 + rnd.nextInt(12);
                BigDecimal valorParcela = total.divide(BigDecimal.valueOf(n), 2, RoundingMode.DOWN);
                BigDecimal acumulado = BigDecimal.ZERO;
                List<String> ps = new ArrayList<>();
                for (int k = 0; k < n; k++) {
                    BigDecimal v = (k == n - 1) ? total.subtract(acumulado) : valorParcela;
                    acumulado = acumulado.add(v);
                    ps.add(v.toPlainString());
                }
                w.println("PARC;" + total.toPlainString() + ";" + n + ";" + String.join(",", ps));
            }
            // TAXA e COM: HALF_UP em 2 casas
            for (int i = 0; i < 150; i++) {
                BigDecimal valor = aleat(2_000_000, 2);
                BigDecimal perc = aleat(10_000, 2);
                w.println("TAXA;" + valor.toPlainString() + ";" + perc.toPlainString() + ";"
                        + valor.multiply(perc).divide(CEM, 2, RoundingMode.HALF_UP).toPlainString());
                BigDecimal qtd = aleat(20_000, 3);
                BigDecimal unit = aleat(99_999, 2);
                BigDecimal total = qtd.multiply(unit); // ItemDocumento.calcularTotal (sem arredondar)
                w.println("COM;" + qtd.toPlainString() + ";" + unit.toPlainString() + ";" + perc.toPlainString() + ";"
                        + total.multiply(perc).divide(CEM, 2, RoundingMode.HALF_UP).toPlainString());
            }
            // META: progresso (percentual ROUND_DOWN, falta nunca negativa)
            for (int i = 0; i < 100; i++) {
                BigDecimal meta = aleat(5_000_000, 2);
                BigDecimal real = i % 7 == 0 ? BigDecimal.ZERO : aleat(8_000_000, 2);
                int pct = real.multiply(BigDecimal.valueOf(100)).divide(meta, 0, RoundingMode.DOWN).intValue();
                BigDecimal falta = meta.subtract(real).max(BigDecimal.ZERO);
                w.println("META;" + meta.toPlainString() + ";" + real.toPlainString() + ";" + pct + ";" + falta.toPlainString() + ";" + (real.compareTo(meta) >= 0));
            }
            // PRECO: preço sugerido na unidade (preço do produto × fator, 2 casas HALF_UP)
            for (int i = 0; i < 60; i++) {
                BigDecimal preco = aleat(99_999, 2);
                BigDecimal fator = aleat(100_000_000L, 6);
                w.println("PRECO;" + preco.toPlainString() + ";" + fator.toPlainString() + ";" + preco.multiply(fator).setScale(2, RoundingMode.HALF_UP).toPlainString());
            }
            // DOC: total do documento = soma dos totais SEM arredondar, arredondada uma vez (e a soma dos itens já arredondados, para o D-05)
            for (int i = 0; i < 100; i++) {
                int n = 1 + rnd.nextInt(6);
                BigDecimal soma = BigDecimal.ZERO, somaArred = BigDecimal.ZERO;
                List<String> itens = new ArrayList<>();
                for (int k = 0; k < n; k++) {
                    BigDecimal qtd = aleat(20_000, 3);
                    BigDecimal valor = aleat(99_999, 2);
                    BigDecimal total = qtd.multiply(valor);
                    soma = soma.add(total);
                    somaArred = somaArred.add(total.setScale(2, RoundingMode.HALF_UP));
                    itens.add(qtd.toPlainString() + "x" + valor.toPlainString());
                }
                w.println("DOC;" + String.join(",", itens) + ";" + soma.setScale(2, RoundingMode.HALF_UP).toPlainString() + ";" + somaArred.toPlainString());
            }
            // MOV: valor movimentado da baixa = valor + juros + multa - desconto
            for (int i = 0; i < 60; i++) {
                BigDecimal valor = aleat(500_000, 2), juros = aleat(5_000, 2), multa = aleat(5_000, 2), desc = aleat(5_000, 2);
                w.println("MOV;" + valor.toPlainString() + ";" + juros.toPlainString() + ";" + multa.toPlainString() + ";" + desc.toPlainString() + ";"
                        + valor.add(juros).add(multa).subtract(desc).toPlainString());
            }
        }
        System.out.println("gerado " + saida);
    }
}
