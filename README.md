# aws-auth-manager

A kuberneres controller to manage the `aws-auth` configmap in EKS using a new `AWSAuthItem` CRD.

The `aws-auth` configmap is used to give RBAC access to IAM users and roles. Because it is a single object, it makes complicated to add and remove entries from multiple sources.

The `aws-auth-manager` provides the ability to define multiple `AWSAuthItem` objects that will be merged to create thew `aws-auth` configmap.

## Example `spec`

```yaml
apiVersion: aws.maruina.k8s/v1alpha1
kind: AWSAuthItem
metadata:
  name: example-one
spec:
  mapRoles:
    - rolearn: arn:aws:iam::111122223333:role/eksctl-my-cluster-nodegroup-standard-wo-NodeInstanceRole-1WP3NUE3O6UCF
      username: system:node:{{EC2PrivateDNSName}}
      groups:
        - system:bootstrappers
        - system:nodes
  mapUsers:
    - userarn: arn:aws:iam::111122223333:user/admin
      username: admin
      groups:
        - system:masters
    - userarn: arn:aws:iam::111122223333:user/ops-user
      username: ops-user
      groups:
        - system:masters
```

## TODO

- Add object `status`
- Add validation webhook for `roleArn` and `userArn`
- More test cases
- Helm chart
- Release
