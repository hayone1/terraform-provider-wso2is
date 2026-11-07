#!/bin/sh
# Init container (WSO2 image). Builds the files the WSO2 entrypoint copies over
# WSO2_SERVER_HOME at start-up (wso2-config-volume and wso2-artifact-volume), and
# exposes the image's dbscripts to the db-init container.
set -eu

JDBC_VERSION="${JDBC_VERSION:-42.7.14}"
JDBC_SHA256="${JDBC_SHA256:-73914527305a40cce504b0d3d90b23caf565136912d607ae8ae7c5895512332c}"

conf=/config-volume/repository/conf
security=/config-volume/repository/resources/security
lib=/artifact-volume/repository/components/lib
mkdir -p "$conf" "$security" "$lib" /dbscripts

cp -R "$WSO2_SERVER_HOME/dbscripts/." /dbscripts/
cp /config/deployment.toml "$conf/deployment.toml"

# TLS keystore from the cert-manager secret (tls.crt, tls.key, ca.crt). The alias
# must match [keystore.tls] alias in deployment.toml.
openssl pkcs12 -export -name wso2tls \
  -inkey /tls/tls.key -in /tls/tls.crt -certfile /tls/ca.crt \
  -out "$security/tls.p12" -passout env:TLS_KEYSTORE_PASSWORD

# The image's truststore plus the internal CA, so the server trusts its own
# certificate when it calls itself on https://localhost:9443.
cp "$WSO2_SERVER_HOME/repository/resources/security/client-truststore.p12" "$security/client-truststore.p12"
keytool -importcert -noprompt -alias wso2is-internal-ca -file /tls/ca.crt \
  -keystore "$security/client-truststore.p12" -storetype PKCS12 -storepass wso2carbon

# PostgreSQL JDBC driver (not shipped with the image).
jar="$lib/postgresql-$JDBC_VERSION.jar"
curl -fsSL --retry 5 -o "$jar" "https://repo1.maven.org/maven2/org/postgresql/postgresql/$JDBC_VERSION/postgresql-$JDBC_VERSION.jar"
echo "$JDBC_SHA256  $jar" | sha256sum -c -
