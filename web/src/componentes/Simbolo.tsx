// Símbolo do Vintém (docs/DESIGN-SYSTEM.md, "Logo"): o disco liso com o V vazado,
// o mesmo de docs/site/src/assets/logo-*.svg. A cor vem de --brand, então segue o
// tema sozinho. É decorativo: o nome "Vintém" vem sempre ao lado, em texto.
export function Simbolo({ tamanho = 24 }: { tamanho?: number }) {
  return (
    <svg viewBox="0 0 64 64" width={tamanho} height={tamanho} aria-hidden="true" focusable="false" style={{ flexShrink: 0, display: "block" }}>
      <path fill="var(--brand)" fillRule="evenodd" d="M32 2a30 30 0 1 1 0 60a30 30 0 1 1 0-60zM17 17h10l5 18 5-18h10L36.5 47h-9z" />
    </svg>
  );
}

// Marca é o símbolo com o nome ao lado, em Inter 600 e na cor de texto: o ouro fica só no símbolo.
export function Marca({ tamanho = 24 }: { tamanho?: number }) {
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-2)", fontWeight: 600, color: "var(--text)" }}>
      <Simbolo tamanho={tamanho} />
      Vintém
    </span>
  );
}
