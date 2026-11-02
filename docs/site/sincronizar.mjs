// Traz para o site os guias que vivem no repositório (fonte única): copia,
// troca o título "#" pelo frontmatter do Starlight e reescreve os links entre
// arquivos para as rotas do site. Gera em src/content/docs/gerado/ (ignorado).
import { mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const raiz = join(dirname(fileURLToPath(import.meta.url)), "../..");
const destino = join(dirname(fileURLToPath(import.meta.url)), "src/content/docs/gerado");
const base = "";

const paginas = [
  ["docs/SANDBOX.md", "sandbox"],
  ["docs/EFI.md", "efi"],
  ["docs/INTER.md", "inter"],
  ["docs/SICOOB.md", "sicoob"],
  ["docs/BB.md", "bb"],
  ["docs/REGRAS-DE-NEGOCIO.md", "regras-de-negocio"],
  ["docs/ARQUITETURA.md", "arquitetura"],
  ["CONTRIBUTING.md", "contribuir"],
  ...readdirSync(join(raiz, "docs/adr")).filter((f) => /^\d{3}-.*\.md$/.test(f)).map((f) => [`docs/adr/${f}`, `adr/${f.replace(/\.md$/, "")}`]),
];
const rota = Object.fromEntries(paginas.map(([arquivo, slug]) => [basename(arquivo), `${base}/gerado/${slug}/`]));

rmSync(destino, { recursive: true, force: true });
for (const [arquivo, slug] of paginas) {
  let texto = readFileSync(join(raiz, arquivo), "utf8");
  const titulo = (texto.match(/^#\s+(.+)$/m)?.[1] ?? slug).replace(/^[^\p{L}\p{N}]+/u, "").trim();
  texto = texto.replace(/^#\s+.+\n+/m, "");
  // Links para outros guias viram rotas do site; o resto aponta para o GitHub.
  texto = texto.replace(/\]\((?!https?:|#|mailto:)([^)\s]+?)(#[^)]*)?\)/g, (m, alvo, ancora = "") => {
    const nome = basename(alvo);
    if (rota[nome]) return `](${rota[nome]}${ancora})`;
    const caminho = join(dirname(arquivo), alvo).replace(/\\/g, "/");
    return `](https://github.com/vintemhq/vintem/blob/develop/${caminho}${ancora})`;
  });
  const saida = join(destino, `${slug}.md`);
  mkdirSync(dirname(saida), { recursive: true });
  writeFileSync(saida, `---\ntitle: ${JSON.stringify(titulo)}\n---\n\n${texto}`);
}
console.log(`${paginas.length} guias sincronizados`);
