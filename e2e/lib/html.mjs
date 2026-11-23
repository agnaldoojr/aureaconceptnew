// Normaliza HTML para comparar a saída do sistema antigo (Thymeleaf) com a do novo (html/template):
// ignora diferenças que não aparecem na tela (ordem e aspas dos atributos, espaços entre tags, comentários, `/>`, entidades).
// Só ferramenta de teste (Node, desenvolvimento).

const ENTIDADES = { quot: '"', amp: '&', lt: '<', gt: '>', apos: "'" };
const LATIN1 = 'nbsp iexcl cent pound curren yen brvbar sect uml copy ordf laquo not shy reg macr deg plusmn sup2 sup3 acute micro para middot cedil sup1 ordm raquo frac14 frac12 frac34 iquest Agrave Aacute Acirc Atilde Auml Aring AElig Ccedil Egrave Eacute Ecirc Euml Igrave Iacute Icirc Iuml ETH Ntilde Ograve Oacute Ocirc Otilde Ouml times Oslash Ugrave Uacute Ucirc Uuml Yacute THORN szlig agrave aacute acirc atilde auml aring aelig ccedil egrave eacute ecirc euml igrave iacute icirc iuml eth ntilde ograve oacute ocirc otilde ouml divide oslash ugrave uacute ucirc uuml yacute thorn yuml'.split(' '); // &iexcl; … &yuml; = U+00A1 … U+00FF (o Thymeleaf escreve assim)
LATIN1.forEach((n, i) => { ENTIDADES[n.toLowerCase() === n ? n : n] = String.fromCharCode(0xa0 + i); });

export function decodificar(s) {
  return s.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (m, e) => {
    if (e[0] === '#') {
      const n = e[1].toLowerCase() === 'x' ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10);
      return String.fromCodePoint(n);
    }
    return ENTIDADES[e] ?? ENTIDADES[e.toLowerCase()] ?? m;
  });
}

function atributos(texto) {
  const out = [];
  const re = /([^\s=\/"'<>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?/g;
  let m;
  while ((m = re.exec(texto))) {
    const valor = m[2] ?? m[3] ?? m[4];
    let v = valor === undefined ? '' : decodificar(valor);
    const nome = m[1].toLowerCase();
    // atributos booleanos: checked, selected, required, readonly, hidden, disabled
    if (['checked', 'selected', 'required', 'readonly', 'hidden', 'disabled'].includes(nome)) v = nome;
    out.push(`${nome}="${v.replace(/\s+/g, ' ').trim()}"`);
  }
  return out.sort();
}

/** Devolve o HTML em linhas estáveis: uma por tag/texto, atributos ordenados, tokens voláteis mascarados. */
export function normalizar(html) {
  let s = html.replace(/<!--[\s\S]*?-->/g, '');
  const partes = [];
  const re = /<\/?[a-zA-Z][^>]*>|[^<]+/g;
  let m;
  while ((m = re.exec(s))) {
    const p = m[0];
    if (p[0] === '<') {
      const t = /^<(\/?)([a-zA-Z0-9]+)([\s\S]*?)\/?>$/.exec(p);
      if (!t) { partes.push(p); continue; }
      const [, fecha, nome, resto] = t;
      if (fecha) { partes.push(`</${nome.toLowerCase()}>`); continue; }
      let attrs = atributos(resto).map((a) =>
        a
          .replace(/(name="_csrf" value=")[^"]*/, '$1X') // vira par ordenado; o valor muda por sessão
          .replace(/-[0-9a-f]{32}(\.[a-z0-9]+)/g, '$1'), // URLs versionadas por hash
      );
      attrs = attrs.map((a) => (/^value="[A-Za-z0-9_-]{40,}"$/.test(a) ? 'value="TOKEN"' : a));
      partes.push(`<${nome.toLowerCase()}${attrs.length ? ' ' + attrs.join(' ') : ''}>`);
    } else {
      // diferença DECIDIDA (D-20, T-21): o novo aplica permissões na hora; o antigo só no próximo login. Os dois textos valem o mesmo para a comparação.
      const txt = decodificar(p).replace(/\s+/g, ' ').trim()
        .replace(/valem a partir do próximo login do usuário\.?|valem na hora(?:, no próximo clique do usuário| para o usuário)\.?/gi, '⟦vigência⟧');
      if (txt) partes.push(txt);
    }
  }
  return partes;
}

/** Pega só a área de conteúdo da tela (o resto é o layout, comparado à parte). */
export function areaConteudo(html) {
  const i = html.indexOf('<main');
  const j = html.indexOf('</main>');
  return i >= 0 && j > i ? html.slice(i, j + 7) : html;
}

export function titulo(html) {
  const m = html.match(/<title>([\s\S]*?)<\/title>/);
  return m ? decodificar(m[1]).trim() : '';
}

/** Primeira diferença entre dois vetores de linhas, com contexto. */
export function primeiraDiferenca(a, b) {
  const n = Math.max(a.length, b.length);
  for (let i = 0; i < n; i++) {
    if (a[i] !== b[i]) {
      return `linha ${i}:\n   antigo: ${a.slice(Math.max(0, i - 1), i + 3).join('\n           ')}\n   novo:   ${b.slice(Math.max(0, i - 1), i + 3).join('\n           ')}`;
    }
  }
  return null;
}
