FROM mirror.gcr.io/library/node:24-slim AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npx vite build

FROM mirror.gcr.io/library/nginx:stable-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/templates/nginx.conf.template
COPY docker-entrypoint.sh /custom-entrypoint.sh

ENV PORT=8080
ENV NGINX_ENVSUBST_OUTPUT_DIR=/etc/nginx

EXPOSE 8080
# Run a thin wrapper first so we can generate env.js at container start.
# The wrapper delegates to nginx's original /docker-entrypoint.sh, and this
# CMD becomes the default arguments passed through to that entrypoint.
ENTRYPOINT ["/custom-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
