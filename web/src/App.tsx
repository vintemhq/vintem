import { useEffect, useState } from "react";
import { sessao } from "./api";
import { Button } from "./componentes/Button";
import { Marca } from "./componentes/Simbolo";
import { CobrancaDetalhe, Cobrancas } from "./telas/Cobrancas";
import { Contas } from "./telas/Contas";
import { Login } from "./telas/Login";
import { VisaoGeral } from "./telas/VisaoGeral";
import { Webhooks } from "./telas/Webhooks";

const rotaAtual = () => location.hash.replace(/^#/, "") || "/";

export function App() {
  const [logado, setLogado] = useState(() => Boolean(sessao.token()));
  const [rota, setRota] = useState(rotaAtual);
  useEffect(() => {
    const mudou = () => setRota(rotaAtual());
    addEventListener("hashchange", mudou);
    return () => removeEventListener("hashchange", mudou);
  }, []);
  const ir = (r: string) => { location.hash = r; };

  if (!logado) return <Login aoEntrar={() => setLogado(true)} />;

  const detalhe = rota.match(/^\/cobrancas\/([A-Za-z0-9]+)$/);
  const menu = [
    { rota: "/", rotulo: "Visão geral" },
    { rota: "/cobrancas", rotulo: "Cobranças" },
    { rota: "/webhooks", rotulo: "Webhooks" },
    { rota: "/contas", rotulo: "Contas" },
  ];
  return (
    <div style={{ display: "grid", gridTemplateColumns: "220px minmax(0, 1fr)", minHeight: "100vh" }}>
      <a href="#conteudo" className="sr-only">Pular para o conteúdo</a>
      <nav aria-label="Principal" style={{ borderRight: "1px solid var(--border)", padding: "var(--space-6) var(--space-4)", display: "flex", flexDirection: "column", gap: "var(--space-2)" }}>
        <div style={{ fontSize: "var(--text-20)", marginBottom: "var(--space-4)" }}><Marca /></div>
        {menu.map((m) => {
          const ativo = m.rota === "/" ? rota === "/" : rota.startsWith(m.rota);
          return (
            <a key={m.rota} href={`#${m.rota}`} aria-current={ativo ? "page" : undefined} style={{ color: ativo ? "var(--text)" : "var(--text-muted)", fontWeight: ativo ? 600 : 400, textDecoration: "none", padding: "var(--space-2) var(--space-3)", borderRadius: "var(--radius-md)", background: ativo ? "var(--surface)" : "transparent" }}>
              {m.rotulo}
            </a>
          );
        })}
        <div style={{ marginTop: "auto" }}>
          <Button variante="ghost" onClick={() => { sessao.sair(); setLogado(false); }}>Sair</Button>
        </div>
      </nav>
      <main id="conteudo" style={{ padding: "var(--space-8)", maxWidth: 1100 }}>
        {detalhe ? <CobrancaDetalhe txid={detalhe[1]!} ir={ir} />
          : rota.startsWith("/cobrancas") ? <Cobrancas ir={ir} />
          : rota.startsWith("/webhooks") ? <Webhooks />
          : rota.startsWith("/contas") ? <Contas />
          : <VisaoGeral ir={ir} />}
      </main>
    </div>
  );
}
