#!/bin/bash

NAME="gerrit"
PORT="8080"
HOST="fboucher.local"

echo "
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${HOST}-deployment
spec:
  rules:
  - host: ${HOST}
    http:
      paths:
      - pathType: Prefix
        path: "/"
        backend:
          service:
            name: ${NAME}
            port:
              number: ${PORT}
" | kubectl apply -f -