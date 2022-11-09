#! env /bin/bash
folder=$1
registry=$2
user=$3
token=$4

  for dir in $folder ; do
    base=$(basename $dir)
    PREF=''
    cat >> $dir/.relok8s-images.yaml <<EOF
---
- "{{ ${PREF}.image.registry }}/{{ ${PREF}.image.repository }}:{{ ${PREF}.image.tag }}"
EOF
    deps=()
    deps+=($(helm dependency list $dir | tail +2 |sed '/^[[:space:]]*$/d'  | awk '{ printf "%s ", $1 }'))
    for i in ${!deps[@]};do
      if [ "${deps[$i]}" == "common" ]; then
          unset deps[$i]
      fi 
    done
    # helm dependency list $dir 2> /dev/null | tail +2 | awk '{ print "helm repo add " $1 " " $3 }' | while read cmd; do bash -c $cmd; done
    for dep in "${deps[@]}" ; do
    echo "- \"{{ .$dep.image.registry }}/{{ .$dep.image.repository }}:{{ .$dep.image.tag }}\"" >> $dir/.relok8s-images.yaml
    done
  version=$(helm show chart $dir | grep ^version: | cut -d : -f 2  | tr -d ' ')
  helm package $1=folder --dependency-update
  relok8s chart move weave-gitops-$version.tgz --registry $registry --repo-prefix $user --out '*.oci.tgz' --yes
  helm push $base-$version.oci.tgz oci://$registry/$user
  COSIGN_EXPERIMENTAL=1 cosign sign $registry/$user/$base:$version
  COSIGN_EXPERIMENTAL=1 cosign verify $registry/$user/$base:$version
  rm -rf $dir/.relok8s-images.yaml $base-$version.tgz $base-$version.oci.tgz
  done
