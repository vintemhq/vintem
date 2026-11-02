import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { centavosDe, formatarCentavos } from "../src/componentes/Money";
import { Marca } from "../src/componentes/Simbolo";
import { estados, StatusBadge } from "../src/componentes/StatusBadge";

describe("Money", () => {
  it("formata centavos em reais", () => {
    expect(formatarCentavos(4990).replace(/\s/g, " ")).toBe("R$ 49,90");
    expect(formatarCentavos(123456789).replace(/\s/g, " ")).toBe("R$ 1.234.567,89");
  });
  it("lê o valor da API Pix sem passar por float", () => {
    expect(centavosDe("49.90")).toBe(4990);
    expect(centavosDe("0.10")).toBe(10);
    expect(centavosDe("1234.5")).toBe(123450);
    expect(centavosDe("15")).toBe(1500);
  });
});

// Os estados de internal/domain (StatusCobranca, StatusCobrancaRecorrente,
// StatusRecorrencia, StatusSolicitacao, StatusTentativa, StatusDevolucao,
// StatusEntrega). Estado novo no domínio precisa entrar aqui e no badge.
const doDominio = [
  "ATIVA", "CONCLUIDA", "REMOVIDA_PELO_USUARIO_RECEBEDOR", "REMOVIDA_PELO_PSP",
  "CRIADA", "CANCELADA", "EXPIRADA", "REJEITADA", "APROVADA",
  "ENVIADA", "RECEBIDA", "ACEITA",
  "AGENDADA", "SOLICITADA", "PAGA",
  "EM_PROCESSAMENTO", "DEVOLVIDO", "NAO_REALIZADO",
  "pendente", "entregue", "dlq",
];

describe("StatusBadge", () => {
  it("tem um badge para cada estado do domínio", () => {
    expect(doDominio.filter((s) => !estados[s])).toEqual([]);
  });
  it("mostra ícone e texto, nunca só cor", () => {
    for (const s of doDominio) {
      const { container, unmount } = render(<StatusBadge status={s} />);
      const icone = container.querySelector('[aria-hidden="true"]');
      expect(icone?.textContent?.trim()).toBeTruthy();
      expect(container.textContent!.replace(icone!.textContent!, "").trim()).toBe(estados[s]!.rotulo);
      unmount();
    }
  });
  it("estado desconhecido aparece como veio, sem quebrar", () => {
    render(<StatusBadge status="NOVO_ESTADO" />);
    expect(screen.getByText("NOVO_ESTADO")).toBeTruthy();
  });
});

describe("Marca", () => {
  it("mostra o nome em texto e o símbolo em --brand, escondido do leitor de tela", () => {
    const { container } = render(<Marca />);
    expect(screen.getByText("Vintém")).toBeTruthy();
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("aria-hidden")).toBe("true");
    expect(svg?.querySelector("path")?.getAttribute("fill")).toBe("var(--brand)");
  });
});
