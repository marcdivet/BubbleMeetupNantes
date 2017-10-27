# BubbleMeetupNantes
## 
La présentation, trois slides n'ont pas été passé :
- la 7 présentant le workflow des déploiements
- la 8 montrant la résistance à la charge de Bubble ... les pics toutes les 1/2 sont causés par la mise à jours des TBB ! la encore la médiane ne bouge pas lorsque les percentile 0,99 et 0,999 présente des variations considérables !
- la 9 les liens vers de la doc

## De quoi rejouer la démo du Meetup Go du 26/10/1017
### Start de minikube (en fonction de votre machine ...)
minikube start --cpus 6 --memory 4096

### Compilation et dockerisation
  - docker build -f Dockerfile.builder -t builder:latest .
  - docker run builder
  - docker ps -all
  - docker container cp idOfContainer:/go/src/app/bubble bubble
  - docker login      
  - docker build -f Dockerfile.production -t marcdivet01/bubble:v001 .
  - docker push votrerepo/bubble:v001

### Déploiement dans minikube
  - kubectl create -f 1grafana.yaml
  - kubectl create -f 2promOperateur.yaml
   --> attendre que le pod de prometheus-operator est bien démaré avant la suite ;-)
  - kubectl create -f 3monitoringClusterBubble.yaml
  - kubectl create -f 4monitorK8s.yaml
  - kubectl create -f 5nodeexport.yaml

Attention : Dans les deux fichiers suivant le répertoire suivant de minikube est
            utilisé : /hosthome/marc/go/src/marc/BubbleV3/partageMinikube/cfgBubble
            Minikube monte votre répetoire home dans /hosthome, c'est donc dans le répertoire si dessus
            qu'il faut copier les fichiers de configuration Personne.json et Contrat.json
  - kubectl create -f 6deployMicSrvPersonne.yaml
  - kubectl create -f 7deployMicSrvContrat.yaml

### Démarage des consoles
  - minikube dashboard
  - minikube service prometheus
  - minikube service graphana

### Configuration de graphana
ajout de la data source : nom --> PrometheusLocalHost url --> http://prometheus:9090
upload des 3 tableaux de bord :
  - Cluster Cockpit-1508946335336.json
  - MicroSrv-1508947462976.json
  - Node Exporter Full-1508947505259.json
  
  
 
