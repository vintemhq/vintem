import { useState } from "react";
import { api, ErroAPI, sessao } from "../api";
import { Button } from "../componentes/Button";
import { Marca } from "../componentes/Simbolo";

export function Login({ aoEntrar }: { aoEntrar: () => void }) {
  const [token, setToken] = useState("");
  const [erro, setErro] = useState("");
  const [carregando, setCarregando] = useState(false);
  return (
    <main style={{ display: "grid", placeItems: "center", minHeight: "100vh", padding: "var(--space-6)" }}>
      <form
        style={{ width: "100%", maxWidth: 420, display: "grid", gap: "var(--space-4)" }}
        onSubmit={async (e) => {
          e.preventDefault();
          setCarregando(true);
          setErro("");
          const agora = new Date();
          try {
            await api(`/cob?inicio=${encodeURIComponent(new Date(agora.getTime() - 60_000).toISOString())}&fim=${encodeURIComponent(agora.toISOString())}`, token.trim());
            sessao.entrar(token.trim());
            aoEntrar();
          } catch (err) {
            setErro(err instanceof ErroAPI && err.status === 401 ? "API key inválida ou revogada." : err instanceof ErroAPI && err.status === 403 ? "Esta API key não tem o escopo cob:read." : "Não foi possível falar com o Vintém.");
          } finally {
            setCarregando(false);
          }
        }}
      >
        <h1 style={{ margin: 0 }}><Marca tamanho={32} /></h1>
        <p className="muted" style={{ margin: 0 }}>Entre com uma API key. O painel só mostra o que os escopos dela permitem.</p>
        <label htmlFor="token" style={{ fontWeight: 600 }}>API key</label>
        <input
          id="token"
          type="password"
          autoComplete="off"
          required
          placeholder="vtm_..."
          value={token}
          onChange={(e) => setToken(e.target.value)}
          aria-describedby={erro ? "erro-login" : "dica-login"}
          style={{ font: "inherit", fontFamily: "var(--font-mono)", padding: "var(--space-2) var(--space-3)", borderRadius: "var(--radius-md)", border: "1px solid var(--border)", background: "var(--bg)", color: "var(--text)" }}
        />
        <div id="dica-login" className="muted" style={{ fontSize: "var(--text-12)" }}>Crie com <code className="mono">vintem apikey criar --nome painel --escopo cob:read</code> ou use a que o <code className="mono">vintem sandbox</code> imprime.</div>
        {erro && <div id="erro-login" role="alert" style={{ color: "var(--danger)" }}>{erro}</div>}
        <Button type="submit" variante="primary" carregando={carregando}>Entrar</Button>
      </form>
    </main>
  );
}
