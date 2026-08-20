FROM node:20-alpine AS anydoc-install
# anydoc CLI supply chain (ADR-0024 §3, AI docs index §5 item 6).
# Pinned version + SRI integrity hash (sha512) verified at build time by npm;
# the hash is the npm registry integrity field of the @firecrawl/anydoc 0.2.0
# tarball. Change the version only together with a matching integrity hash.
ARG ANYDOC_VERSION=0.2.0
ARG ANYDOC_INTEGRITY=sha512-Hm4xkqioU12jrt18asY1WKX22ATR4mFHQ4HOFsWEwCMFZWaf9nDj2rbApZPPcNZPGKYB4mrLLJTK91XwLNZkFA==
RUN npm install -g @firecrawl/anydoc@${ANYDOC_VERSION} --registry=https://registry.npmjs.org
RUN node -e "const {createHash}=require('crypto');const fs=require('fs');const p=require('child_process').execSync('npm pack @firecrawl/anydoc@${ANYDOC_VERSION}').toString().trim();const s=fs.readFileSync(p);const got='sha512-'+createHash('sha512').update(s).digest('base64');if(got!=$ANYDOC_INTEGRITY){console.error('anydoc integrity mismatch: got '+got+' want '+$ANYDOC_INTEGRITY);process.exit(1)};console.log('anydoc integrity verified: '+p)"

FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /bin/bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:latest
WORKDIR /root/
COPY --from=anydoc-install /usr/local/lib/node_modules/@firecrawl/anydoc /usr/local/lib/node_modules/@firecrawl/anydoc
COPY --from=anydoc-install /usr/local/bin/anydoc /usr/local/bin/anydoc
ENV ANYDOC_BIN_PATH=/usr/local/bin/anydoc
COPY --from=builder /bin/api .
COPY --from=builder /bin/bootstrap-admin .
EXPOSE 8080

CMD ["./api"]