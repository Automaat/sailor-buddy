FROM node:24-alpine

RUN apk add --no-cache openjdk17-jre-headless

RUN npm install -g firebase-tools

WORKDIR /opt/firebase

COPY firebase.json .

EXPOSE 9099 4000

CMD ["firebase", "emulators:start", "--project", "sailor-buddy-dev", "--export-on-exit", "data", "--import", "data"]
