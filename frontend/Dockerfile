FROM node:24-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:24-alpine AS dev
WORKDIR /app
COPY package*.json ./
RUN npm ci
ENV HOST=0.0.0.0
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "5173"]

FROM node:24-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000
COPY --from=builder /app/build ./build
COPY --from=builder /app/package.json .
EXPOSE 3000
CMD ["node", "build"]
