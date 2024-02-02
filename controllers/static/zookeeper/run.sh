#!/bin/bash

set -a
export HOME=/data
ROOT=/zookeeper
ZK_LOG_LEVEL="INFO"
ZK_DATA_DIR="/data"
ZK_DATA_LOG_DIR="/data/log"
ZK_CONF_DIR="/conf"
ZK_CLIENT_PORT=${ZK_CLIENT_PORT:-2181}
ZK_SSL_CLIENT_PORT=${ZK_SSL_CLIENT_PORT:-2281}
ZK_SERVER_PORT=${ZK_SERVER_PORT:-2888}
ZK_ELECTION_PORT=${ZK_ELECTION_PORT:-3888}
ZK_TICK_TIME=${ZK_TICK_TIME:-2000}
ZK_INIT_LIMIT=${ZK_INIT_LIMIT:-10}
ZK_SYNC_LIMIT=${ZK_SYNC_LIMIT:-5}
ZK_MAX_HEAP_SIZE=${ZK_MAX_HEAP_SIZE:-2G}
ZK_MIN_HEAP_SIZE=${ZK_MIN_HEAP_SIZE:-128M}
ZK_MAX_CLIENT_CNXNS=${ZK_MAX_CLIENT_CNXNS:-60}
ZK_MIN_SESSION_TIMEOUT=${ZK_MIN_SESSION_TIMEOUT:- $((ZK_TICK_TIME*2))}
ZK_MAX_SESSION_TIMEOUT=${ZK_MAX_SESSION_TIMEOUT:- $((ZK_TICK_TIME*20))}
ZK_SNAP_RETAIN_COUNT=${ZK_SNAP_RETAIN_COUNT:-3}
ZK_PURGE_INTERVAL=${ZK_PURGE_INTERVAL:-0}
JMXPORT=1099
JMXSSL=false
JMXAUTH=false
JMXDISABLE=${JMXDISABLE:-false}
ID_FILE="$ZK_DATA_DIR/myid"
ZK_CONFIG_FILE="$ZK_CONF_DIR/zoo.cfg"
HOST=$(hostname)
DOMAIN=$(hostname -d)
JVMFLAGS="-Xmx$ZK_MAX_HEAP_SIZE -Xms$ZK_MIN_HEAP_SIZE"

APPJAR=$(echo $ROOT/*jar)
CLASSPATH="${ROOT}/lib/*:${APPJAR}:${ZK_CONF_DIR}:"

if [[ $HOST =~ (.*)-([0-9]+)$ ]]; then
    NAME=${BASH_REMATCH[1]}
    ORD=${BASH_REMATCH[2]}
    MY_ID=$((ORD+1))
else
    echo "Failed to extract ordinal from hostname $HOST"
    exit 1
fi

mkdir -p "$ZK_DATA_LOG_DIR"
echo $MY_ID >> "$ID_FILE"

if [[ -f /tls/server/ca.crt ]]; then
  cp /tls/server/ca.crt /data/server-ca.pem
  cat /tls/server/tls.crt /tls/server/tls.key > /data/server.pem
fi
if [[ -f /tls/client/ca.crt ]]; then
  cp /tls/client/ca.crt /data/client-ca.pem
  cat /tls/client/tls.crt /tls/client/tls.key > /data/client.pem
fi

cat << EOF >> "$ZK_CONFIG_FILE"
dataDir=$ZK_DATA_DIR
dataLogDir=$ZK_DATA_LOG_DIR
tickTime=$ZK_TICK_TIME
initLimit=$ZK_INIT_LIMIT
syncLimit=$ZK_SYNC_LIMIT
maxClientCnxns=$ZK_MAX_CLIENT_CNXNS
minSessionTimeout=$ZK_MIN_SESSION_TIMEOUT
maxSessionTimeout=$ZK_MAX_SESSION_TIMEOUT
autopurge.snapRetainCount=$ZK_SNAP_RETAIN_COUNT
autopurge.purgeInterval=$ZK_PURGE_INTERVAL
4lw.commands.whitelist=*
EOF

# Client TLS configuration
if [[ -f /tls/client/ca.crt ]]; then

cat << EOF >> "$ZK_CONFIG_FILE"
secureClientPort=$ZK_SSL_CLIENT_PORT
ssl.keyStore.location=/data/client.pem
ssl.trustStore.location=/data/client-ca.pem
EOF

else
  echo "clientPort=$ZK_CLIENT_PORT" >> "$ZK_CONFIG_FILE"
fi

# Server TLS configuration
if [[ -f /tls/server/ca.crt ]]; then

cat << EOF >> "$ZK_CONFIG_FILE"
serverCnxnFactory=org.apache.zookeeper.server.NettyServerCnxnFactory
sslQuorum=true
ssl.quorum.keyStore.location=/data/server.pem
ssl.quorum.trustStore.location=/data/server-ca.pem
EOF

fi

for (( i=1; i<=ZK_REPLICAS; i++ ))
do
    echo "server.$i=$NAME-$((i-1)).$DOMAIN:$ZK_SERVER_PORT:$ZK_ELECTION_PORT" >> "$ZK_CONFIG_FILE"
done

cp /config-scripts/logback.xml /conf

if [ -n "$JMXDISABLE" ]
then
    MAIN=org.apache.zookeeper.server.quorum.QuorumPeerMain
else
    MAIN="-Dcom.sun.management.jmxremote -Dcom.sun.management.jmxremote.port=$JMXPORT -Dcom.sun.management.jmxremote.authenticate=$JMXAUTH -Dcom.sun.management.jmxremote.ssl=$JMXSSL -Dzookeeper.jmx.log4j.disable=$JMXLOG4J org.apache.zookeeper.server.quorum.QuorumPeerMain"
fi

set -x
exec java -cp "$CLASSPATH" $JVMFLAGS $MAIN $ZK_CONFIG_FILE
