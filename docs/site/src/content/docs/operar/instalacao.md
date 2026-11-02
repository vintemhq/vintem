---
title: Instalação
description: Os três jeitos de instalar o Vintém, com a imagem Docker, os binários do release ou o go install.
---

| Jeito | Painel | Para quê |
|---|---|---|
| [Imagem Docker](#imagem-docker) | Sim | Produção, o caminho recomendado |
| [Binário do release](#binário-do-release) | Sim | Servidor sem Docker |
| [`go install`](#go-install) | Não | Experimentar ou desenvolver |

Para só testar, sem conta no banco, veja [Do zero ao Pix pago](/comecar/quickstart/).

## Imagem Docker

A imagem é multi-arch (amd64 e arm64) e roda sem root, com um binário só.

```bash
docker run --rm ghcr.io/vintemhq/vintem:0.11.1 version
```

Use sempre uma versão fixa. Para subir em produção, siga o [guia de produção](/operar/producao/).

## Binário do release

Os [releases](https://github.com/vintemhq/vintem/releases) trazem binários para Linux,
macOS e Windows, em amd64 e arm64, e um `checksums.txt`. Baixe e confira antes de usar:

```bash
VERSAO=0.11.1
curl -LO https://github.com/vintemhq/vintem/releases/download/v$VERSAO/vintem_${VERSAO}_linux_amd64.tar.gz
curl -LO https://github.com/vintemhq/vintem/releases/download/v$VERSAO/checksums.txt
sha256sum -c --ignore-missing checksums.txt
tar xzf vintem_${VERSAO}_linux_amd64.tar.gz
./vintem version
```

O `sha256sum` precisa responder `OK`. No macOS, use `shasum -a 256 -c --ignore-missing checksums.txt`.
No Windows, o pacote é um `.zip`.

## go install

Com Go 1.27 ou mais novo:

```bash
go install github.com/vintemhq/vintem/cmd/vintem@latest
```

:::caution
O binário instalado assim vem **sem o painel** e mostra `dev` em `vintem version`. Em
produção, prefira a imagem ou os binários do release.
:::

## Depois de instalar

- [Colocar em produção](/operar/producao/)
- [Todas as variáveis de ambiente](/operar/configuracao/)
- [Todos os comandos](/operar/cli/)
