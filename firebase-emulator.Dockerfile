FROM node:24-alpine

RUN apk add --no-cache openjdk17-jre-headless

ARG FIREBASE_TOOLS_VERSION=13.35.1

RUN npm install -g firebase-tools@${FIREBASE_TOOLS_VERSION}

WORKDIR /opt/firebase

COPY firebase.json .

EXPOSE 9099 4000

CMD ["firebase", "emulators:start", "--project", "sailor-buddy-dev", "--export-on-exit", "data", "--import", "data"]
