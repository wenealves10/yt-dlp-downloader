#!/usr/bin/env sh
# ============================================================================
# Builda as imagens do AdVideo NO PRÓPRIO SERVIDOR.
#
# Não há registry: as imagens ficam locais, com a tag :latest, e é por isso que
# o DEPLOY_REVISION do manifesto existe — ele é o que faz o Swarm recriar os
# containers depois de um build bem-sucedido.
#
# Buildar aqui, e não na máquina de desenvolvimento, é decisão de arquitetura: o
# servidor é ARM. Imagem x86 carregada lá morre em `exec format error`, e
# buildar com buildx emulado leva horas — a imagem do navegador sozinha passa de
# 1 GB e instala o Chrome inteiro.
#
# Uso, a partir da raiz do repositório:
#   sh docker/builder/build-images.sh                    # todas
#   sh docker/builder/build-images.sh api worker         # só estas
#   ENV_FILE=outro/lugar/stack.env sh docker/builder/build-images.sh
#
# A validação roda ANTES do primeiro build: descobrir que faltou uma variável
# depois de vinte minutos baixando o Chrome é uma espera que não se paga.
# ============================================================================
set -eu

RAIZ="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="${ENV_FILE:-$RAIZ/docker/builder/stack.env}"
TAG="${TAG:-latest}"
REGISTRY_PREFIX="${REGISTRY_PREFIX:-}"

vermelho() { printf '\033[31m%s\033[0m\n' "$*" >&2; }
amarelo()  { printf '\033[33m%s\033[0m\n' "$*" >&2; }
verde()    { printf '\033[32m%s\033[0m\n' "$*"; }
titulo()   { printf '\n\033[1m== %s\033[0m\n' "$*"; }

if [ ! -f "$ENV_FILE" ]; then
    vermelho "Arquivo de configuração não encontrado: $ENV_FILE"
    vermelho "  cp docker/builder/stack.env.example docker/builder/stack.env"
    vermelho "  chmod 600 docker/builder/stack.env"
    exit 1
fi

# shellcheck disable=SC1090
set -a
. "$ENV_FILE"
set +a

image_name() {
    if [ -n "$REGISTRY_PREFIX" ]; then
        printf '%s/%s:%s' "$REGISTRY_PREFIX" "$1" "$TAG"
    else
        printf '%s:%s' "$1" "$TAG"
    fi
}

API_IMAGE="$(image_name advideo-api)"
WORKER_IMAGE="$(image_name advideo-worker)"
BROWSER_IMAGE="$(image_name advideo-browser)"
APP_IMAGE="$(image_name advideo-app)"
MIGRATE_IMAGE="$(image_name advideo-migrate)"

# ---------------------------------------------------------------------------
# Validação
# ---------------------------------------------------------------------------
FALTOU=0

exigir() {
    valor="$(printenv "$1" || true)"
    if [ -z "$valor" ]; then
        vermelho "  FALTA  $1 — $2"
        FALTOU=1
    fi
}

avisar() {
    valor="$(printenv "$1" || true)"
    if [ -z "$valor" ]; then
        amarelo "  vazio  $1 — $2"
    fi
}

titulo "conferindo $ENV_FILE"

# O servidor é ARM e as imagens não têm registry nem multi-arch: elas são
# construídas para a máquina em que este script roda.
ARQ="$(uname -m)"
if [ "$ARQ" != "aarch64" ] && [ "$ARQ" != "arm64" ]; then
    amarelo "  ATENÇÃO: esta máquina é $ARQ, e o servidor é ARM (aarch64)."
    amarelo "           A imagem gerada aqui NÃO roda lá. Rode este script no servidor."
fi

exigir DEPLOY_REVISION "é o que faz o Swarm recriar os containers depois deste build"
exigir APP_DOMAIN      "sai na regra do Traefik e DENTRO do bundle do front"
exigir API_DOMAIN      "sai na regra do Traefik e DENTRO do bundle do front"

exigir TOKEN_PASETO_KEY "chave de sessão; sem ela a API não sobe"
exigir REDIS_PASSWORD   "o Redis da stack sobe com requirepass"

# O PASETO v2.local usa a chave como bytes crus de uma cifra de 256 bits. Com
# tamanho diferente a API sobe e só falha no primeiro login.
if [ -n "${TOKEN_PASETO_KEY:-}" ]; then
    TAMANHO="$(printf %s "$TOKEN_PASETO_KEY" | wc -c | tr -d ' ')"
    if [ "$TAMANHO" != "32" ]; then
        vermelho "  RUIM   TOKEN_PASETO_KEY tem $TAMANHO caracteres, precisa de 32 — gere com: openssl rand -hex 16"
        FALTOU=1
    fi
fi

# Sem banco externo, o da stack é obrigatório.
if [ -z "${EXTERNAL_DB_SOURCE:-}" ]; then
    exigir POSTGRES_USER     "usuário do Postgres da stack"
    exigir POSTGRES_DB       "nome do banco"
    exigir POSTGRES_PASSWORD "senha do Postgres da stack"
fi

# As senhas entram dentro de URLs postgres://usuario:SENHA@host/banco e
# redis://:SENHA@host. Um caractere reservado no meio quebra a URL, e o erro que
# aparece é "authentication failed" — que manda procurar no lugar errado.
for var in POSTGRES_PASSWORD REDIS_PASSWORD; do
    valor="$(printenv "$var" || true)"
    case "$valor" in
        *[@:/\#\?\&\'\"\$\ ]*)
            vermelho "  RUIM   $var tem caractere que quebra a URL — gere com: openssl rand -hex 24"
            FALTOU=1
            ;;
    esac
done

exigir ACCESS_KEY_ID     "credencial do bucket R2"
exigir SECRET_ACCESS_KEY "credencial do bucket R2"
exigir BUCKET_NAME       "bucket de mídia"
exigir ENDPOINT_URL      "endpoint do R2"
exigir BUCKET_HOST       "host público do bucket; entra no bundle do front"

exigir TURNSTILE_SECRET    "sem ele o cadastro e o login recusam todo mundo"
exigir TURNSTILE_SITE_KEY  "entra no bundle do front"

# Chave de teste em produção deixa o cadastro aberto para qualquer robô.
case "${TURNSTILE_SITE_KEY:-}" in
    1x00000000000000000000*)
        vermelho "  RUIM   TURNSTILE_SITE_KEY é a chave de TESTE, que aprova qualquer um"
        FALTOU=1
        ;;
esac

exigir BROWSER_SERVICE_TOKEN "segredo compartilhado entre API, worker e navegador remoto"
if [ -n "${BROWSER_SERVICE_TOKEN:-}" ]; then
    TAMANHO="$(printf %s "$BROWSER_SERVICE_TOKEN" | wc -c | tr -d ' ')"
    if [ "$TAMANHO" -lt 32 ]; then
        vermelho "  RUIM   BROWSER_SERVICE_TOKEN tem só $TAMANHO caracteres — gere com: openssl rand -hex 32"
        FALTOU=1
    fi
fi

# O Swarm ignora security_opt, então não há como entregar o perfil seccomp que o
# sandbox do Chrome precisa. Com o sandbox ligado, o navegador não abre.
if [ "${BROWSER_DISABLE_SANDBOX:-}" != "true" ]; then
    vermelho "  RUIM   BROWSER_DISABLE_SANDBOX=${BROWSER_DISABLE_SANDBOX:-<vazio>} — em Swarm precisa ser true, ou o navegador não abre"
    FALTOU=1
fi

# Proxy ligado sem URL faz o yt-dlp receber --proxy vazio e falhar todo download.
if [ "${PROXY_ENABLED:-false}" = "true" ] && [ -z "${PROXY_URL:-}" ]; then
    vermelho "  RUIM   PROXY_ENABLED=true sem PROXY_URL"
    FALTOU=1
fi

# O proxy de resolução é pago por volume. Ligar o proxy GERAL junto manda o
# vídeo inteiro por ele e queima a cota em poucos downloads — que é exatamente
# o que a separação das duas variáveis existe para evitar.
if [ -n "${RESOLVE_PROXY_URL:-}" ] && [ "${PROXY_ENABLED:-false}" = "true" ]; then
    amarelo "  ATENÇÃO  RESOLVE_PROXY_URL e PROXY_ENABLED=true juntos: a mídia vai sair pelo proxy geral e a cota do residencial não será poupada"
fi

avisar SUPER_ADMIN_EMAIL "sem ele ninguém acessa o painel de contas do YouTube no primeiro deploy"

# ---------------------------------------------------------------------------
# Conferência do manifesto
#
# A interpolação do Compose NÃO é recursiva: em `${A:-texto${B}}` o default
# fecha no primeiro `}` e o resto vira literal. O sintoma é uma URL montada pela
# metade, e ela só aparece quando o container sobe — não no deploy.
# ---------------------------------------------------------------------------
MANIFESTO="$RAIZ/docker/builder/docswarm.yaml"

if grep -q '\${[^}]*\${' "$MANIFESTO" 2>/dev/null; then
    vermelho "  RUIM   $MANIFESTO tem \${...\${...}} aninhado — a interpolação do Compose não é recursiva:"
    grep -n '\${[^}]*\${' "$MANIFESTO" | sed 's/^/         /' >&2
    FALTOU=1
fi

# Renderiza o manifesto com este mesmo ambiente e confere as URLs montadas. Um
# `${` sobrando dentro de uma URL é exatamente o sintoma acima.
if docker stack config -c "$MANIFESTO" >/tmp/advideo-render.$$ 2>/dev/null; then
    if grep -E '://[^[:space:]]*\$\{' /tmp/advideo-render.$$ >/dev/null 2>&1; then
        vermelho "  RUIM   o manifesto renderizado tem URL com variável não substituída:"
        grep -E '://[^[:space:]]*\$\{' /tmp/advideo-render.$$ | sed -E 's/:[^:@]{8,}@/:<senha>@/' | sed 's/^/         /' >&2
        FALTOU=1
    fi
    rm -f /tmp/advideo-render.$$
else
    rm -f /tmp/advideo-render.$$
    amarelo "  aviso  não deu para renderizar o manifesto com \`docker stack config\` (siga assim)"
fi

if [ "$FALTOU" -ne 0 ]; then
    vermelho ""
    vermelho "não vou buildar com a configuração errada. Corrija e rode de novo."
    exit 1
fi
verde "  ok — nada faltando"

# ---------------------------------------------------------------------------
# Build
#
# Um de cada vez, de propósito: o box é pequeno e paralelizar aqui só faz cada
# imagem demorar mais.
# ---------------------------------------------------------------------------
cd "$RAIZ"

# Com o buildx moderno o builder ativo pode ser do driver `docker-container`, e
# nesse caso a imagem fica só no cache: ela não entra no store local e o Swarm
# não a encontra, com o build tendo terminado sem erro. `--load` resolve, e é
# aceito também pelo driver `docker` padrão, onde é o comportamento implícito.
DOCKER_BUILD="docker build"
if docker buildx version >/dev/null 2>&1; then
    DOCKER_BUILD="docker build --load"
fi

build_migrate() {
    titulo "$MIGRATE_IMAGE"
    $DOCKER_BUILD -f api/Dockerfile.migrate -t "$MIGRATE_IMAGE" api
}

build_api() {
    titulo "$API_IMAGE"
    $DOCKER_BUILD -f api/Dockerfile.server -t "$API_IMAGE" api
}

build_worker() {
    titulo "$WORKER_IMAGE (ffmpeg + yt-dlp + Deno)"
    $DOCKER_BUILD -f api/Dockerfile.worker -t "$WORKER_IMAGE" api
}

build_browser() {
    titulo "$BROWSER_IMAGE (Chrome + Xvfb — é a maior, passa de 1 GB)"
    $DOCKER_BUILD -f api/Dockerfile.browser -t "$BROWSER_IMAGE" api
}

# O front é o único cujo build depende do stack.env: os domínios são gravados
# dentro do JavaScript. Trocar APP_DOMAIN ou API_DOMAIN obriga a rebuildar aqui.
build_app() {
    titulo "$APP_IMAGE (bundle com API_DOMAIN=$API_DOMAIN)"
    $DOCKER_BUILD -f app/Dockerfile \
        --build-arg "VITE_API_URL=https://${API_DOMAIN}" \
        --build-arg "VITE_BUCKET_HOST=${BUCKET_HOST}" \
        --build-arg "VITE_SITE_KEY=${TURNSTILE_SITE_KEY}" \
        -t "$APP_IMAGE" app
}

if [ "$#" -eq 0 ]; then
    set -- migrate api worker browser app
fi

for alvo in "$@"; do
    case "$alvo" in
        migrate) build_migrate ;;
        api)     build_api ;;
        worker)  build_worker ;;
        browser) build_browser ;;
        app)     build_app ;;
        *) vermelho "alvo desconhecido: $alvo (use migrate, api, worker, browser, app)"; exit 1 ;;
    esac
done

titulo "pronto"
echo "Imagens criadas:"
echo "  $MIGRATE_IMAGE"
echo "  $API_IMAGE"
echo "  $WORKER_IMAGE"
echo "  $BROWSER_IMAGE"
echo "  $APP_IMAGE"

# Não é aritmética garantida: DEPLOY_REVISION pode ser um rótulo qualquer, e uma
# conta que falha aqui derrubaria o script DEPOIS de todos os builds terem dado
# certo.
case "$DEPLOY_REVISION" in
    ''|*[!0-9]*) PROXIMA_REVISAO="algo diferente de $DEPLOY_REVISION" ;;
    *)           PROXIMA_REVISAO=$((DEPLOY_REVISION + 1)) ;;
esac

cat <<FIM

Próximo passo, e ele NÃO é opcional:

  1. incremente DEPLOY_REVISION em $ENV_FILE
     (está em ${DEPLOY_REVISION} — vá para ${PROXIMA_REVISAO})
  2. Portainer > Stacks > advideo > Editor
  3. Environment variables > Load variables from .env file > $ENV_FILE
  4. Update the stack

Sem o passo 1, o Swarm olha para a mesma tag :latest, conclui que nada mudou e
mantém o código velho no ar — com o build tendo terminado sem um único erro.
FIM
