// Compara o ESTADO FINAL do banco do sistema antigo (SQL Server AureaRef) com o do novo (SQLite) nas tabelas de despejo-tabelas.json.
// Uso: node despejo.mjs <arquivo-sqlite-do-novo>   (sai com 1 se houver diferença). Só desenvolvimento.
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = dirname(fileURLToPath(import.meta.url));
const espec = JSON.parse(readFileSync(resolve(aqui, 'despejo-tabelas.json'), 'utf8'));
const tabelas = Object.entries(espec).filter(([n]) => !n.startsWith('_')).map(([n, v]) => [n, Array.isArray(v) ? v : v.cols, Array.isArray(v) ? '' : v.onde, Array.isArray(v) ? '' : v.ordem]);
const esperadas = new Set((process.env.DESPEJO_ESPERA || '').split(',').map((s) => s.trim()).filter(Boolean)); // tabelas com diferença DECIDIDA neste roteiro
const bancoNovo = process.argv[2];
if (!bancoNovo) { console.error('informe o arquivo SQLite do novo'); process.exit(2); }

function expressao(c) {
  const [nome, tipo] = c.split(':');
  if (tipo === 'd') return `ISNULL(CONVERT(NVARCHAR(10), ${nome}, 23), 'NULL')`;
  if (tipo && /^\d+$/.test(tipo)) return `ISNULL(CAST(CAST(ROUND(${nome} * ${10 ** Number(tipo)}, 0) AS BIGINT) AS NVARCHAR(40)), 'NULL')`;
  return `ISNULL(CAST(${nome} AS NVARCHAR(4000)), 'NULL')`;
}

function despejoAntigo() {
  const saida = {};
  for (const [tabela, cols, onde, ordem] of tabelas) {
    const sql = `SET NOCOUNT ON; SELECT ${cols.map(expressao).join(" + '|' + ")} FROM ${tabela} ${onde ? 'WHERE ' + onde : ''} ORDER BY ${ordem || cols[0].split(':')[0]}`;
    const txt = execFileSync('sqlcmd', ['-S', 'localhost' + String.fromCharCode(92) + 'SQLEXPRESS', '-U', 'sa', '-P', '123', '-C', '-d', 'AureaRef', '-f', '65001', '-h', '-1', '-W', '-b', '-Q', sql], { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
    saida[tabela] = txt.split(/\r?\n/).filter((l) => l.length > 0);
  }
  return saida;
}

function despejoNovo() {
  const txt = execFileSync('go', ['run', './tools/despejo', '-db', bancoNovo, '-tabelas', resolve(aqui, 'despejo-tabelas.json')], { encoding: 'utf8', cwd: resolve(aqui, '..'), maxBuffer: 64 * 1024 * 1024 });
  const saida = {};
  let atual = null;
  for (const l of txt.split(/\r?\n/)) {
    if (l.startsWith('## ')) { atual = l.slice(3); saida[atual] = []; } else if (l.length && atual) saida[atual].push(l);
  }
  return saida;
}

const a = despejoAntigo();
const n = despejoNovo();
let diferencas = 0;
for (const [tabela] of tabelas) {
  const x = a[tabela] || [], y = n[tabela] || [];
  const problemas = [];
  const max = Math.max(x.length, y.length);
  for (let i = 0; i < max && problemas.length < 6; i++) {
    if (x[i] !== y[i]) problemas.push(`   linha ${i + 1}:\n     antigo: ${x[i] ?? '(não existe)'}\n     novo:   ${y[i] ?? '(não existe)'}`);
  }
  if (problemas.length && esperadas.has(tabela)) console.log(`≈ ${tabela}: antigo ${x.length} linhas, novo ${y.length} (diferença decidida)`);
  else if (problemas.length) { diferencas++; console.log(`✗ ${tabela}: antigo ${x.length} linhas, novo ${y.length}\n${problemas.join('\n')}`); }
  else console.log(`✓ ${tabela}: ${x.length} linhas iguais`);
}
console.log(`\nEstado final do banco: ${diferencas ? diferencas + ' tabela(s) com diferença' : 'todas as ' + tabelas.length + ' tabelas iguais'}.`);
process.exit(diferencas ? 1 : 0);
