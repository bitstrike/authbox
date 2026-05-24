# Build stage for Go application
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/authbox ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache \
    openldap \
    openldap-back-mdb \
    openldap-overlay-syncprov \
    openldap-clients \
    ca-certificates \
    tini

# LDAP data and config directories
RUN mkdir -p /var/lib/openldap/openldap-data \
    /etc/openldap/slapd.d \
    /var/run/openldap \
    /data

# Copy Go binary
COPY --from=builder /bin/authbox /usr/local/bin/authbox

# Copy entrypoint
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Persistent volumes
# /data - SQLite database, CA keys, exports
# /var/lib/openldap - LDAP database
# /etc/openldap/slapd.d - LDAP cn=config
VOLUME ["/data", "/var/lib/openldap", "/etc/openldap/slapd.d"]

# LDAPS (636) and HTTPS API/UI (8443)
EXPOSE 389 636 8443

ENTRYPOINT ["tini", "--"]
CMD ["/entrypoint.sh"]
