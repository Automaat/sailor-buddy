FROM node:26-alpine

RUN apk add --no-cache openjdk17-jre-headless

ARG FIREBASE_TOOLS_VERSION=15.36.0

RUN npm install -g firebase-tools@${FIREBASE_TOOLS_VERSION}

WORKDIR /opt/firebase

COPY firebase.json .

EXPOSE 9099 4000

# data/ is the compose volume mount point. The export deletes and replaces
# its target directory, which fails with "Resource busy" on a mount point,
# so export to a subdirectory inside the volume instead.
CMD ["firebase", "emulators:start", "--project", "sailor-buddy-dev", "--export-on-exit", "data/export", "--import", "data/export"]
