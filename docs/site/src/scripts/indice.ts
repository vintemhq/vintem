// O índice "Nesta página" destaca o título que chega ao topo da tela. O título de uma
// seção curta no fim da página nunca chega lá: o destaque fica preso no anterior, e o
// clique no índice não leva até ele. Por isso, a página ganha no fim só o espaço que
// falta para o último título alcançar o topo.
function ajustarFim() {
  const principal = document.querySelector<HTMLElement>("main");
  const links = document.querySelectorAll<HTMLAnchorElement>("starlight-toc a, mobile-starlight-toc a");
  const ultimo = links[links.length - 1];
  if (!principal || !ultimo) return;
  const alvo = document.getElementById(decodeURIComponent(ultimo.hash.slice(1)));
  if (!alvo) return;

  principal.style.paddingBottom = "";
  const margemDoTitulo = parseFloat(getComputedStyle(alvo).scrollMarginTop) || 0;
  const topo = alvo.getBoundingClientRect().top + scrollY - margemDoTitulo;
  const falta = topo + innerHeight - document.documentElement.scrollHeight;
  if (falta <= 0) return;
  principal.style.paddingBottom = `${Math.ceil(falta)}px`;
}

function irAoTitulo() {
  if (!location.hash) return;
  document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView();
}

ajustarFim();
irAoTitulo();
addEventListener("resize", ajustarFim);
addEventListener("load", () => {
  ajustarFim();
  irAoTitulo();
});
document.fonts?.ready.then(ajustarFim);
