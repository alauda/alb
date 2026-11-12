# Inputs and export

## Required objects

Export the migration-related objects of all five kinds:

- `ALB2` objects for the ALBs being migrated;
- `Frontend` objects;
- `Rule` objects;
- `Namespace` objects for the namespaces referenced by the migration; do not
  export all namespaces;
- original `Ingress` objects only for the sources referenced by
  `Rule.spec.source`; do not export all Ingresses.

`Rule` is the key input: it tells the converter which ALB, Frontend, source
Ingress, backend namespaces, and certificate references are involved. Use it
to build the targeted export list.

Not required:

- `Service` objects: Frontend/Rule already contain Service name and port;
- `Secret` contents: Frontend/Rule already contain `certificate_name`;
- all namespaces, all Ingresses, all Secrets.

The generator writes `missing-resources.md` and a read-only `fetch-missing.sh`
for the missing related objects. The assembler is complete only after those
files are added to the input and the case is regenerated.

The generator accepts multi-document YAML and `kind: List` exports. Keep
`metadata.namespace`, `metadata.name`, `metadata.labels`, and
`ownerReferences` intact so Frontend/Rule/ALB associations remain resolvable.

```shell
kubectl get alaudaloadbalancer2 -A -o yaml > alb2.yaml
kubectl get frontends.crd.alauda.io -A -o yaml > frontends.yaml
kubectl get rules.crd.alauda.io -A -o yaml > rules.yaml

# Only namespaces referenced by the migration, not all namespaces:
kubectl get namespace <related-ns> -o yaml > namespace.yaml

# Only source Ingresses referenced by Rule.spec.source:
kubectl get ingress -n <source-ns> <source-name> -o yaml > source-ingress.yaml
```

## Do not export

Do not put kubeconfig, tokens, Secret `data`/`stringData`, certificate private
keys, or credentials in the input directory. A TLS Secret name is enough for
conversion; the Secret content is not needed.

## Relationship fields

The generator uses:

- `alb2.cpaas.io/name` or `ownerReferences[kind=ALB2]` to associate objects;
- `alb2.cpaas.io/frontend` on Rule to associate a Rule with a Frontend;
- `spec.source.type: ingress` to identify Ingress-generated Rules;
- `spec.certificate_name` for certificate references;
- `spec.serviceGroup.services[]` for backend Service references.

If you anonymize an export, preserve these relationships and the namespace /
name graph; otherwise the generated Routes may attach to the wrong Gateway or
backend.

## Version

This directory targets ALB 4.3. Record the source ALB version and the target
Gateway API / Envoy Gateway version in the review notes.
