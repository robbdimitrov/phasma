# syntax=docker/dockerfile:1
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . .
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
COPY --from=builder --chown=node:node /app/build ./build
COPY --from=builder --chown=node:node /app/package.json ./package.json
USER node
ENV NODE_ENV=production
ENV PORT=8080
ENV BODY_SIZE_LIMIT=1100K
EXPOSE 8080
CMD ["node", "build"]
