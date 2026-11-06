# 🎨 Vintém — Design system

## Conceito

**Vintém** cuida do dinheiro de quem o usa. A identidade tem que passar
**solidez e calma**: quem abre o painel quer saber, em dois segundos, se o
dinheiro entrou e se alguma coisa está quebrada. Nada de gradiente de fintech,
nada de animação comemorativa. Ouro aparece como acento da marca, não como cor
de interface.

## Tokens

Vivem em `web/src/design/tokens.css` como CSS custom properties. Componente
nunca usa cor literal.

### Cor

| Token | Claro | Escuro | Uso |
|---|---|---|---|
| `--bg` | `#FFFFFF` | `#0B1120` | Fundo |
| `--surface` | `#F8FAFC` | `#0F172A` | Cards, tabelas |
| `--border` | `#E2E8F0` | `#1E293B` | Divisórias |
| `--text` | `#0F172A` | `#F1F5F9` | Texto principal |
| `--text-muted` | `#475569` | `#94A3B8` | Texto secundário |
| `--brand` | `#946C08` | `#D4A017` | Ouro: logo, foco, link |
| `--success` | `#047857` | `#10B981` | Pago, concluído, devolvido |
| `--warning` | `#B45309` | `#F59E0B` | Ativa, pendente, em processamento |
| `--danger` | `#DC2626` | `#F87171` | Falha, DLQ, removida, rejeitada |
| `--info` | `#2563EB` | `#60A5FA` | Recorrência, informativo |

Todas as cores de texto passam **WCAG AA** (4,5:1) sobre `--bg` e `--surface`,
nos dois temas, e `web/test/contraste.test.ts` confere isso a cada build. No tema
claro, o ouro é `#946C08` e o verde é `#047857`: os tons anteriores (`#B8860B`,
3,25:1, e `#059669`, 3,77:1) não passavam, e o teste é que mostrou.

### Tipografia

- **Inter** — interface. Escala: 12 · 14 · 16 · 20 · 24 · 32.
- **JetBrains Mono** — txid, e2eid, chaves, JSON, BR Code. Sempre com botão de copiar.
- Números de dinheiro em `font-variant-numeric: tabular-nums`.

### Espaço, raio, sombra

- Espaço em escala de 4px: `--space-1` (4) até `--space-12` (48).
- Raio: `--radius-sm` 4px · `--radius-md` 8px · `--radius-lg` 12px.
- Sombra só em elemento flutuante (menu, modal). Card tem borda, não sombra.

## Logo

Vintém era uma moeda, e o símbolo é uma: um disco em `--brand` com o V vazado, que
deixa o fundo aparecer. A versão com serrilha na borda é só para tamanhos grandes, como
a abertura do site; o favicon e o cabeçalho usam o disco liso, que continua legível em
16 px. O nome vem ao lado, em Inter 600 e na cor de texto: o ouro fica só no símbolo.

| Arquivo (em `docs/site`) | Uso |
|---|---|
| `src/assets/logo-claro.svg` e `logo-escuro.svg` | Cabeçalho |
| `src/assets/moeda-claro.svg` e `moeda-escuro.svg` | Abertura da home |
| `public/favicon.svg` | Favicon; acompanha o tema do navegador |
| `public/favicon-32.png` | Favicon de reserva |
| `public/apple-touch-icon.png` | Tela inicial do iPhone |
| `public/icon-512.png` | Tamanho grande, para perfis e divulgação |

No painel, o símbolo é o componente `Simbolo` (`web/src/componentes/Simbolo.tsx`), com o
mesmo desenho do `logo-*.svg` e a cor de `--brand`, e `Marca` põe o nome ao lado. O
favicon, o de reserva e o do iPhone são cópias dos de `docs/site/public`, em `web/public`:
quem trocar um troca o outro.

## Componentes base

| Componente | Regra |
|---|---|
| `Button` | `primary` · `secondary` · `ghost` · `danger`; estado de loading sem mudar largura |
| `StatusBadge` | Um por estado de domínio (`ATIVA`, `CONCLUIDA`, `DEVOLVIDO`...). **Ícone + texto**, nunca só cor |
| `Money` | Recebe centavos (`number`), formata `Intl.NumberFormat('pt-BR', {style:'currency', currency:'BRL'})`. Único jeito de mostrar dinheiro |
| `CopyField` | Texto mono + copiar + feedback "copiado" acessível (`aria-live`) |
| `QRCode` | BR Code renderizado + "Pix Copia e Cola" logo abaixo |
| `Table` | Ordenável, paginação por cursor, linha clicável abre o detalhe |
| `Card` / `Stat` | Número grande + rótulo + variação |
| `EmptyState` | Sempre com a próxima ação ("Crie sua primeira cobrança no sandbox") |
| `Timeline` | Histórico de estados e de tentativas de webhook |

## Telas do MVP

1. **Visão geral** — recebido hoje / no mês, cobranças ativas, webhooks na DLQ,
   certificados vencendo e **"você economizou R$ X em taxas"** (estimativa
   contra a tabela de taxas dos gateways, com fonte e data).
2. **Cobranças** — lista + detalhe (QR, estados, Pix recebido, devoluções).
3. **Recorrências** — Pix Automático: status, próximas cobranças, tentativas.
4. **Webhooks** — entregas, tentativas, DLQ com reenvio.
5. **Contas** — bancos conectados, capacidades, certificado e validade.
6. **API keys** — criar (mostra uma vez), escopos, revogar.

## Acessibilidade

- Navegação inteira por teclado, foco visível em `--brand`.
- Status nunca depende só de cor (ícone + texto).
- Respeita `prefers-color-scheme` e `prefers-reduced-motion`.
