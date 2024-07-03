# X100 Operator Upgrade 
Following Resources can be updated
1. Controller
2. Manifests 
3. CRD

## 1. Controllers 
Changes in the controller are incorporated in the form of .go files. Steps to update the controller are :
```
    1. Make changes to the .go files present in the controller folder.
    2. Update the version of the x100-operator , in the Makefile
    3. Make the new x100-operator image.
    4. Apply the changes by using command oc/kc apply -f x100-operator-deploy.yaml
```
The old operator will be gracefully terminated and the new one will be created. The pods will keep on running and will not be terminated.

## 2. Manifest
The Manifests are present in the Asset folder.

### Assets are divided into :
1. Init-tasks
2. Templates

#### For Init-tasks updates :

All the Manifests present in the Pre-Requisites are:
1. Role
2. Role Binding
3. Persistant Volumes
3. Persistant Volume Claims
4. Service Account

To Update the Role/Role Binding/Persistant Volumes/Persistant Volume Claim . Following steps are required:
```
    1. Make changes to the Manifests. (Make sure to update all the dependencies before applying the update)
    2. Update the version of the x100-operator
    3. Make the new x100-operator
    4. Apply the changes by using command oc/kc apply -f x100-operator-deploy.yaml
```
If same name of the manifest is there in the new version, then no updates will be observed. If name is updated the new resorces are created and old ones will 
still be present. On deleting the manifest both new and old resources will also be deleted.

> NOTE:- Updates to the ServiceAccount cannot be done without pods termination.

#### For Template Updates:
The Resources under Template are :
1. Firmware
2. Hw-manager 
3. Device-plugin
4. Kernel Modules

The update to the template resources cannot be done as the pods that are alrea

#### Config:
For updates in config , steps to follow are:
```
    1. Make changes to the Manifests.
    2. Update the version of the x100-operator
    3. Make the new x100-operator
    4. Apply the changes by using command oc/kc apply -f x100-operator-deploy.yaml
```

## 3. CRD
Currently updates to the CRD is not supported as it requires the running pods to be terminated first.
