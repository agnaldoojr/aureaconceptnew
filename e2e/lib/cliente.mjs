// Cliente HTTP mínimo para testar o sistema (antigo ou novo) como um navegador faria:
// guarda cookies, lê o token CSRF dos formulários, segue (ou não) redirecionamentos e lê as mensagens flash.
// Só ferramenta de desenvolvimento/teste: não entra na imagem de produção.

export class Cliente {
  constructor(base) {
    this.base = base.replace(/\/$/, '');
    this.jar = new Map();
    this.csrf = null;
  }

  cookieHeader() {
    return [...this.jar].map(([k, v]) => `${k}=${v}`).join('; ');
  }

  guardarCookies(res) {
    const lista = typeof res.headers.getSetCookie === 'function' ? res.headers.getSetCookie() : [];
    for (const c of lista) {
      const [kv] = c.split(';');
      const i = kv.indexOf('=');
      const k = kv.slice(0, i).trim();
      const v = kv.slice(i + 1).trim();
      if (/max-age=0/i.test(c) || /expires=thu, 01 jan 1970/i.test(c)) this.jar.delete(k);
      else this.jar.set(k, v);
    }
  }

  /** Faz uma requisição. `form` vira application/x-www-form-urlencoded (com _csrf); `json` vira JSON. */
  async req(caminho, { method = 'GET', form, json, headers = {}, semCsrf = false } = {}) {
    const h = { ...headers };
    const cookie = this.cookieHeader();
    if (cookie) h.cookie = cookie;
    let body;
    if (form) {
      const p = new URLSearchParams();
      for (const [k, v] of Object.entries(form)) {
        if (Array.isArray(v)) v.forEach((x) => p.append(k, x ?? ''));
        else if (v !== undefined && v !== null) p.append(k, v);
      }
      if (this.csrf && !semCsrf && !p.has('_csrf')) p.append('_csrf', this.csrf);
      body = p.toString();
      h['content-type'] = 'application/x-www-form-urlencoded';
    } else if (json !== undefined) {
      body = JSON.stringify(json);
      h['content-type'] = 'application/json';
    }
    const res = await fetch(this.base + caminho, { method, headers: h, body, redirect: 'manual' });
    this.guardarCookies(res);
    const texto = await res.text();
    return { status: res.status, loc: res.headers.get('location'), texto, headers: res.headers };
  }

  /** GET que segue redirecionamentos (até 5) e devolve a página final. */
  async pagina(caminho) {
    let r = await this.req(caminho);
    for (let i = 0; i < 5 && r.status >= 300 && r.status < 400 && r.loc; i++) {
      const loc = r.loc.replace(this.base, '');
      r = await this.req(loc.startsWith('http') ? new URL(loc).pathname + new URL(loc).search : loc);
    }
    return r;
  }

  lerCsrf(html) {
    const m = html.match(/name="_csrf"\s+value="([^"]+)"/) || html.match(/value="([^"]+)"\s+name="_csrf"/);
    if (m) this.csrf = m[1];
    return this.csrf;
  }

  /** Entra pelo formulário de login. Devolve true se caiu fora de /login. */
  async entrar(login, senha, { modoApp = '0' } = {}) {
    this.jar.clear();
    this.csrf = null;
    const tela = await this.req('/login');
    this.lerCsrf(tela.texto);
    const r = await this.req('/login', { method: 'POST', form: { login, senha, modoApp } });
    const ok = r.status >= 300 && r.status < 400 && r.loc && !/\/login/.test(r.loc);
    if (ok) {
      // o token CSRF muda com a nova sessão: pega de qualquer formulário gerado pelo servidor
      const f = await this.pagina('/produtos/novo');
      if (!this.lerCsrf(f.texto)) {
        const g = await this.pagina('/unidades/novo');
        this.lerCsrf(g.texto);
      }
    }
    return ok;
  }

  /** POST de formulário que segue o redirecionamento e devolve { status, loc, flash:{sucesso,erro,info} }. */
  async enviar(caminho, form) {
    const r = await this.req(caminho, { method: 'POST', form });
    let destino = null;
    let flash = {};
    if (r.status >= 300 && r.status < 400 && r.loc) {
      const url = new URL(r.loc, this.base);
      destino = url.pathname + url.search;
      const pg = await this.req(destino);
      flash = Cliente.alertas(pg.texto);
      if (pg.status >= 300 && pg.status < 400 && pg.loc) destino += ' -> ' + pg.loc;
    } else {
      flash = Cliente.alertas(r.texto);
    }
    return { status: r.status, loc: destino, flash, texto: r.texto };
  }

  static alertas(html) {
    const out = {};
    const re = /class="alert alert-(success|error|info)"[\s\S]*?<span>([^<]*)<\/span>/g;
    let m;
    while ((m = re.exec(html))) {
      const chave = { success: 'sucesso', error: 'erro', info: 'info' }[m[1]];
      out[chave] = m[2].replace(/&amp;/g, '&').replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').trim();
    }
    return out;
  }

  /** Opções (value, texto) de um <select name="..."> de uma página. */
  static opcoes(html, nome) {
    const i = html.search(new RegExp(`<select[^>]*name="${nome.replace(/\./g, '\\.')}"`));
    if (i < 0) return [];
    const fim = html.indexOf('</select>', i);
    const trecho = html.slice(i, fim);
    const lista = [];
    const re = /<option\s+([^>]*)>([^<]*)<\/option>/g;
    let m;
    while ((m = re.exec(trecho))) {
      const v = (m[1].match(/value="([^"]*)"/) || [])[1];
      if (v === undefined || v === '') continue;
      lista.push({ valor: v, texto: m[2].replace(/&amp;/g, '&').replace(/&#39;/g, "'").replace(/&quot;/g, '"').trim() });
    }
    return lista;
  }
}

export function iso(d) {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

export function somarDias(d, n) {
  const x = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  x.setDate(x.getDate() + n);
  return x;
}
