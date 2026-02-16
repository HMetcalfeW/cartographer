#!/bin/sh

# Add Helm repos from HELM_REPOS env var
# Format: "name=url,name2=url2"
if [ -n "$HELM_REPOS" ]; then
  IFS=','
  for repo in $HELM_REPOS; do
    name="${repo%%=*}"
    url="${repo#*=}"
    helm repo add "$name" "$url" >/dev/null 2>&1
  done
  helm repo update >/dev/null 2>&1
fi

exec cartographer "$@"
