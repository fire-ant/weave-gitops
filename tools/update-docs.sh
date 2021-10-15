# Note that GITOPS_VERSION and ALGOLIA_API_KEY environment variables must be set
# before running this script

WEAVE_GITOPS_BINARY=$1
WEAVE_GITOPS_DOC_REPO=$2

cd $WEAVE_GITOPS_DOC_REPO/docs
yarn install
# update version information
ex - installation.md << EOS
/download\/
%s,download/\(.*\)/,download/${GITOPS_VERSION}/,
/Current Version
.,+4! ${WEAVE_GITOPS_BINARY} version
+5d
wq!
EOS
# create CLI reference
${WEAVE_GITOPS_BINARY} docs
git add *.md
git rm -f --ignore-unmatch cli-reference.md
ex - gitops.md << EOS
1i
---
sidebar_position: 3
---
 # CLI Reference
 .
wq!
EOS
# create versioned docs
cd $WEAVE_GITOPS_DOC_REPO
version_number=$(cut -f2 -d'v' <<< $GITOPS_VERSION)
npm run docusaurus docs:version $version_number
