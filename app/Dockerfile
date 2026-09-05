# Stage 1: Build
FROM node:18-alpine AS builder

WORKDIR /app

# Diferente da API, o front é um bundle estático: estas variáveis são gravadas
# DENTRO do JavaScript no momento do build. Trocar o domínio da API exige
# reconstruir esta imagem — não basta atualizar a stack.
#
# Elas chegam como variáveis de processo, que o Vite prioriza sobre os arquivos
# .env do repositório.
ARG VITE_API_URL
ARG VITE_BUCKET_HOST
ARG VITE_SITE_KEY
ENV VITE_API_URL=$VITE_API_URL \
    VITE_BUCKET_HOST=$VITE_BUCKET_HOST \
    VITE_SITE_KEY=$VITE_SITE_KEY

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

# Stage 2: Deploy
FROM nginx:alpine AS production

# Remove conteúdo padrão
RUN rm -rf /usr/share/nginx/html/*

# Copia build do frontend
COPY --from=builder /app/dist /usr/share/nginx/html

# Copia a configuração customizada do Nginx
COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
