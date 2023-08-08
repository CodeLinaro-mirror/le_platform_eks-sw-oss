#!/bin/bash
#******************************************************************************
# Copyright (c) 2023 Qualcomm Technologies, Inc.
# All Rights Reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc.
#******************************************************************************/

function check_file_errors()
{
   if [[ -f eks-x100-ns.yaml && \
         -f eks-x100-secret.yaml && \
         -f eks-x100-sa.yaml && \
         -f eks-x100-pv.yaml && \
         -f eks-x100-pvc.yaml && \
         -f eks-x100-fwpod.yaml && \
         -f eks-x100-hwmgrpod.yaml && \
         -f eks-x100-driverspod.yaml && \
         -f eks-x100-devplugin.yaml ]]; then
      echo "all the required manifests exist"
   else
      echo "ERROR: required manifest files are not present, exiting without creating resources!"
      exit 1;
   fi

}

function create_resources()
{

   check_file_errors
   echo "Creating csm-x100 Resources"

   #namespace
   NAMESPACE=x100-operator-resources
   if [ kubectl get namespace $NAMESPACE > /dev/null 2>&1 ]; then
      echo "Namespace $NAMESPACE already exists"
   else
      #Create the namespace
      kubectl create -f eks-x100-ns.yaml
      # Check if the namespace was created successfully
      if [ $? -eq 0 ]; then
         echo " $NAMESPACE Namespace created successfully"
      else
         echo "ERROR: namespace $NAMESPACE creation failed"
      fi
   fi

   #secret
   SECRET=csm-x100-eks-pullsecret
   if [ kubectl get secret $SECRET -n $NAMESPACE > /dev/null 2>&1 ]; then
      echo "Secret $SECRET already exists in namespace $NAMESPACE"
   else
      #Create the secret
      kubectl create -f eks-x100-secret.yaml
      # Check if the secret was created successfully
      if [ $? -eq 0 ]; then
         echo "Secret $SECRET created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: secret $SECRET creation failed"
      fi
   fi

   #Serviceaccount
   SERVICEACCOUNT=csm-x100boot-eks-sa
   if [ kubectl get serviceaccount $SERVICEACCOUNT -n $NAMESPACE > /dev/null 2>&1 ]; then
      echo "Serviceaccount $SERVICEACCOUNT already exists in namespace $NAMESPACE"
   else
      #Create the service account
      kubectl create -f eks-x100-sa.yaml
      # Check if the service account was created successfully
      if [ $? -eq 0 ]; then
         echo "ServiceAccount $SERVICEACCOUNT created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: serviceaccount $SERVICEACCOUNT creation failed"
      fi
   fi

   # persistent volume
   PERSISTENTVOLUME=csm-x100boot-eks-pv
   if [ kubectl get pv $PERSISTENTVOLUME > /dev/null 2>&1 ]; then
      echo "Persistent volume $PERSISTENTVOLUME already exists"
   else
      #Create the persistent volume
      kubectl create -f eks-x100-pv.yaml
      # Check if the persistent volume was created successfully
      if [ $? -eq 0 ]; then
         echo "PersistentVolume $PERSISTENTVOLUME created successfully"
      else
         echo "ERROR: persistentvolume $PERSISTENTVOLUME creation failed"
      fi
   fi

   # PersistentVolumeClaim
   PVC=csm-x100boot-eks-pvc
   if [ kubectl get pvc $PVC -n $NAMESPACE > /dev/null 2>&1 ]; then
      echo "PersistentVolumeClaim $PVC already exists in namespace $NAMESPACE"
   else
      #Create the persistent volume claim
      kubectl create -f eks-x100-pvc.yaml
      # Check if the persistent volume claim was created successfully
      if [ $? -eq 0 ]; then
         echo "$PVC created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: PVC $PVC creation failed"
      fi
   fi

   #Firmware Pod
   FWDS=csm-x100fwocp-daemonset
   if [ kubectl get ds $FWDS -n $NAMESPACE > /dev/null 2>&1 ]; then
      echo "Firmware DS $FWDS already exists in namespace $NAMESPACE"
   else
      #Create the firmware pod
      kubectl create -f eks-x100-fwpod.yaml
      # Check if the Firmwar-pod was created successfully
      if [ $? -eq 0 ]; then
         echo "$FWDS created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: $FWDS pod creation failed"
      fi
   fi

   # HwMgr Pod
   HWMGRDS=csm-x100-eks-hwmgrdaemonset
   if [ kubectl get ds $HWMGRDS -n $NAMESPACE > /dev/null 2>&1 ] ; then
      echo "$HWMGRDS already exists in namespace $NAMESPACE"
   else
      #Create the hardware pod
      kubectl create -f eks-x100-hwmgrpod.yaml
      # Check if the hardware manager pod was created successfully
      if [ $? -eq 0 ]; then
         echo "$HWMGRDS created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: $HWMGRDS pod creation failed"
      fi
   fi

   # Drivers Pod
   DRIVERDS=csm-x100-eks-kmodulesdaemonset
   if [ kubectl get ds $DRIVERDS -n $NAMESPACE > /dev/null 2>&1 ]; then
      echo "$DRIVERDS already exists in namespace $NAMESPACE"
   else
      kubectl wait --for=condition=ready pod -l app=csm-x100-eks-hwmgrdaemonset -n x100-operator-resources
      kubectl create -f eks-x100-driverspod.yaml
      # Check if the hardware manager pod was created successfully
      if [ $? -eq 0 ]; then
         echo "$DRIVERSDS created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: $DRIVERSDS pod creation failed"
      fi
   fi

   # Device Plugin Pod
   DEVPLUGINDS=csm-x100-device-plugin
   if [ kubectl get ds $DEVPLUGINDS -n $NAMESPACE > /dev/null 2>&1 ]; then
      echo "$DEVPLUGINDS already exists in namespace $NAMESPACE"
   else
      kubectl wait --for=condition=ready pod -l app=csm-x100-eks-kmodulesdaemonset -n x100-operator-resources
      kubectl create -f eks-x100-devplugin.yaml
      # Check if the device plugin pod was created successfully
      if [ $? -eq 0 ]; then
         echo "$DEVPLUGINDS created successfully in namespace $NAMESPACE"
      else
         echo "ERROR: $DEVPLUGINDS pod creation failed"
      fi
   fi


   # Check the status of the pods
   kubectl get pods -n $NAMESPACE
}

function delete_resources()
{
   kubectl delete  -f eks-x100-devplugin.yaml \
                   -f eks-x100-driverspod.yaml \
                   -f eks-x100-hwmgrpod.yaml \
                   -f eks-x100-fwpod.yaml \
                   -f eks-x100-pvc.yaml \
                   -f eks-x100-pv.yaml \
                   -f eks-x100-sa.yaml \
                   -f eks-x100-secret.yaml \
                   -f eks-x100-ns.yaml;
}

function get_resources()
{
   kubectl get pods -o wide -n x100-operator-resources
}

while getopts ":cdg" opt;
do
   case $opt in
   c)
      create_resources
      ;;
   d)
      echo "Deleting Resources"
      delete_resources
      ;;
   g)
      echo "Getting the Status of Pods"
      get_resources
      ;;
   *)
      echo "Invalid option: -$OPTARG"
      echo "Usage: "
      echo "./eks-x100-deploy.sh -c for Creating"
      echo "./eks-x100-deploy.sh -d for Deleting"
      echo "./eks-x100-deploy.sh -g for getting the Status of the Pods"
      ;;
   esac
done
echo "INFO: Re-run with -g option to view the pods status"
echo "INFO: Run with -d option to delete the k8s resources"
echo "INFO: Run with -c option to create the k8s resources"
