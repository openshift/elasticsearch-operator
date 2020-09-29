# Troubleshooting

## Kibana

### Why am I unable to see infrastructure logs
Infrastructure logs are visible from the `Global` tenant and require `administrator` permissions. See the [access control](access-control.md) documentation for additional information about how a user is determined to have the `administrator` role.
### kube:admin is unable to see infrastructure logs
`kube:admin` by default does not have the correct permissions to be given the admin role.   See the [access control](access-control.md) documentation for additional informations.  You may grant the permissions by:
```
oc adm policy add-cluster-role-to-user cluster-admin kube:admin
```