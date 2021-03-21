
# build the docker image and submit it to container registry
gcloud builds submit --config cloudbuild.yaml

# list built image from above
gcloud container images list-tags gcr.io/pong-proof-of-concept/pongperfwide-test


# get clusters network
gcloud container clusters describe pongperfwide-1 --format=get"(network)"

# get clusters IPv4 CIDR
gcloud container clusters describe pongperfwide-1 --format=get"(clusterIpv4Cidr)"

# use network and ipv4 addr to create firewall rule to access other vms in project (i.e. access p4 vm's)
gcloud compute firewall-rules create "pongperfwide-1-to-all-vms-on-network" --network="default" --source-ranges="10.60.0.0/14" --allow=tcp,udp,icmp,esp,ah,sctp


----


# spin up cluster 
gcloud beta container --project "pong-proof-of-concept" clusters create "pongperfwide-1" --zone "us-east4-c" --no-enable-basic-auth --cluster-version "1.15.9-gke.9" --machine-type "n1-standard-1" --image-type "UBUNTU" --disk-type "pd-standard" --disk-size "100" --metadata disable-legacy-endpoints=true --scopes "https://www.googleapis.com/auth/devstorage.read_only","https://www.googleapis.com/auth/logging.write","https://www.googleapis.com/auth/monitoring","https://www.googleapis.com/auth/servicecontrol","https://www.googleapis.com/auth/service.management.readonly","https://www.googleapis.com/auth/trace.append" --num-nodes "2" --enable-stackdriver-kubernetes --enable-ip-alias --network "projects/pong-proof-of-concept/global/networks/default" --subnetwork "projects/pong-proof-of-concept/regions/us-east4/subnetworks/default" --default-max-pods-per-node "110" --addons HorizontalPodAutoscaling,HttpLoadBalancing --no-enable-autoupgrade --enable-autorepair

# get credentials from cluster
gcloud container clusters get-credentials --zone "us-east4-c" pongperfwide-1

# apply storage types
kubectl apply -f ./pongperfwide-pv.yaml

# deploy the statefulset
kubectl apply -f ./pongperfwide-deployment.yaml

# describe the statefulset
kubectl describe statefulset pongperfwide

# list pods in deployment
kubectl get pods -l app=pongperfwide

# get log from a pod (index 0 in this case)
kubectl logs pongperfwide-0
kubectl logs pongperfwide-0 -c pongperfwide

# scale the set to X replicas
kubectl scale statefulsets pongperfwide --replicas=<new-replicas>



