# AdVideo — Docker Swarm Builder (servidor ARM)

Esta pasta contém o build das cinco imagens e a stack de produção para o
Portainer/Docker Swarm. PostgreSQL e Redis são contêineres da própria stack, sem
porta publicada; o armazenamento de arquivos continua no Cloudflare R2.

## Arquivos

- `stack.env.example`: modelo do único arquivo de configuração da produção.
- `build-images.sh`: monta as cinco imagens lendo o mesmo arquivo, e **recusa
  buildar** se faltar variável obrigatória ou se alguma estiver errada.
- `docswarm.yaml`: stack que deve ser colada/enviada ao Portainer.

Os Dockerfiles continuam em `api/` e `app/` — é onde o `compose.dev.yaml` os lê.
Movê-los para cá quebraria o ambiente de desenvolvimento.

## Build no manager do Swarm

Na raiz do repositório:

```sh
cp docker/builder/stack.env.example docker/builder/stack.env
chmod 600 docker/builder/stack.env
# edite docker/builder/stack.env e preencha todos os valores
sh docker/builder/build-images.sh
```

Só uma imagem: `sh docker/builder/build-images.sh api worker`.

**Builde sempre no servidor.** Ele é ARM e não há registry: imagem construída num
PC x86 e carregada lá morre em `exec format error`, um erro que não menciona
arquitetura nenhuma. Nenhum Dockerfile fixa `GOARCH`, e o repositório do Chrome
e o download do Deno resolvem a arquitetura em tempo de build — as imagens saem
corretas para a máquina onde o script roda, seja ela ARM ou x86.

A imagem do navegador passa de 1 GB e instala o Chrome inteiro; é a demorada.

## O bundle do front é a exceção

Ao contrário da API, o front é estático: `APP_DOMAIN`, `API_DOMAIN`,
`BUCKET_HOST` e `TURNSTILE_SITE_KEY` são gravados **dentro do JavaScript** no
momento do build.

Trocar qualquer um dos quatro exige `sh docker/builder/build-images.sh app`.
Atualizar a stack sozinho não muda um bundle que já foi gerado — a tela continua
falando com o domínio antigo, sem nenhum erro no deploy.

## Deploy

No Portainer, crie/atualize a stack pelo editor web usando `docswarm.yaml` como
manifesto e importe o conteúdo de `stack.env` na seção **Environment variables**.

Depois de todo rebuild, **incremente `DEPLOY_REVISION`**: as imagens são
`:latest` e locais, e sem essa mudança o Swarm mantém os containers antigos no
ar.

A rede `traefik-public` precisa existir antes do primeiro deploy — ela é criada
pela stack do Traefik, fora desta.

## Primeiro deploy

1. Aponte `APP_DOMAIN` e `API_DOMAIN` no DNS para o servidor.
2. Suba a stack. `migrations-advideo` termina em `complete`; é o esperado.
3. Cadastre o super admin pela tela de registro do próprio produto.
4. Preencha `SUPER_ADMIN_EMAIL` com esse e-mail, incremente `DEPLOY_REVISION` e
   atualize a stack. A API promove o usuário no start e loga
   `bootstrap: usuário promovido a super admin`.
5. Esvazie `SUPER_ADMIN_EMAIL` no próximo deploy: ele existe só para o bootstrap.
6. Entre em **YouTube → Contas**, adicione a conta e faça o login no navegador
   remoto. O painel detecta a sessão sozinho.

## Banco externo

Para usar Supabase ou outro Postgres gerenciado, preencha `EXTERNAL_DB_SOURCE` e
remova o serviço `postgres-advideo` do manifesto. As migrations continuam
rodando pela stack, contra a URL que você informou.

## O que NÃO está aqui

`docker-stack/deploy-*.yml`, na raiz, é o deploy antigo com imagens do
`ghcr.io` para o ambiente x86. Os dois caminhos não devem conviver no mesmo
servidor: escolha um. Este é o do servidor ARM.
