# Stage 1: Build
FROM node:18-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:alpine as production
# Remove conteúdo padrão
RUN rm -rf /usr/share/nginx/html/*
# Copia build do Flutter
COPY --from=builder /app/dist /usr/share/nginx/html
# Copia a configuração customizada
COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]