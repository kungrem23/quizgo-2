# syntax=docker/dockerfile:1
FROM node:24-alpine AS build
WORKDIR /app
ARG VITE_GAME_ORIGIN
ENV VITE_GAME_ORIGIN=${VITE_GAME_ORIGIN}
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . .
RUN npm run build

FROM nginx:1.28-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
