package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type (
	LigneCmd struct {
		ipPort         string
		nomPod         string
		nomNode        string
		nomService     string
		versionService string
		nbProc         int
		cfgPath        string
	}

	ParamExec struct {
		MinPollGoroutines   int `json:"minPollGoroutines"`
		MaxPoolGoroutines   int `json:"maxPoolGoroutines"`
		NbPasParCycle       int `json:"nbPasParCycle"`
		DureeParPasSec      int `json:"dureeParPasSec"`
		NbTriDansBubble     int `json:"nbTriDansBubble"`
		DureeWaitDansBubble int `json:"dureeWaitDansBubble"`
	}
)

var (
	wg sync.WaitGroup

	// Configuration saisie en ligne de commande
	ligneCmd LigneCmd
	// Configuration de la charge (par scrutation du fichier de configuration)
	paramExec ParamExec

	//	Summary (percentile)
	server_duration_perc = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "Server_duration_perc",
			Help:       "Temps d'execution mesuré par le serveur - Percentils",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 0.999: 0.0001},
		},
		[]string{"Pod", "NomNode", "NomMicSrv", "NomVerMicSrv", "MetMicSrv"},
	)

	//	Histogram ( LinearBuckets ou ExponentialBuckets)
	serveur_duration_histo = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "Serveur_duration_histo",
		Help: "Temps d'execution mesuré par le serveur - Histogram ",
		// 200 buckets de pas de 5 ms en expérimentation seulement ...
		Buckets: prometheus.LinearBuckets(0, 0.005, 200),
	},
		[]string{"Pod", "NomNode", "NomMicSrv", "NomVerMicSrv", "MetMicSrv"},
	)

	//workergauge (bouge en plus et en moins, ex mesure de température)
	server_call_gauge_count = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "Server_call_gauge_count",
		Help: "Nombre de requêtes instantanées recues par le serveur",
	},
		[]string{"Pod", "NomNode", "NomMicSrv", "NomVerMicSrv"},
	)
	//Counter (compte, donc monte toujours , ex nombre de requête HTTP...)
	server_call_counter_count = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "Server_call_counter_count",
		Help: "Compte le nombre de requêtes recues par le serveur",
	},
		[]string{"Pod", "NomNode", "NomMicSrv", "NomVerMicSrv", "MetMicSrv"},
	)
	//ce qu'il ne faut pas faire
	server_actif_gauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "Server_actif_gauge",
		Help: "à 1 lors du démarage set à 0 à l'arret du serveur et celà ne sert à rien !!! pourquoi?...",
	},
		[]string{"Pod", "NomNode", "NomMicSrv", "NomVerMicSrv"},
	)
)

func init() {

	// Initialisation des mesures ...
	prometheus.MustRegister(server_duration_perc)
	prometheus.MustRegister(serveur_duration_histo)
	prometheus.MustRegister(server_call_gauge_count)
	prometheus.MustRegister(server_call_counter_count)
	prometheus.MustRegister(server_actif_gauge)
}

func readFileConfig() (pe ParamExec) {
	//Ouverture du fichier
	jsonFile, err := os.Open(ligneCmd.cfgPath + ligneCmd.nomService + ".json")
	if err != nil {
		fmt.Println("Error opening JSON file:", err)
		panic("le fichier nomservice.json doit être dans" + ligneCmd.cfgPath)
	}
	//lecture du contenu du fichier dans jsonData
	jsonData, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("Error reading JSON data:", err)
		panic("Error reading JSON data")
	}
	//désérialisation du json dans la structure pe
	json.Unmarshal(jsonData, &pe)
	//return PE (voir la déclaration de la fonction
	return
}

func readconfig(fileChange chan bool) {
	// cette fonction va être active en tâche de fond
	// son role est scruter le fichier de config
	// si il change mettre à jour la structure paramExec
	for {
		//pour toujours ... scrutation toutes les 5 sec.
		time.Sleep(5 * time.Second)
		pe := readFileConfig()
		if pe != paramExec {
			paramExec = pe

			fmt.Printf("Changement ! dureeParPasSec (value): %v\n", paramExec.DureeParPasSec)
			fmt.Printf("Changement ! dureeWaitDansBubble (value): %v\n", paramExec.DureeWaitDansBubble)
			fmt.Printf("Changement ! maxPoolGoroutines (value): %v\n", paramExec.MaxPoolGoroutines)
			fmt.Printf("Changement ! minPollGoroutines (value): %v\n", paramExec.MinPollGoroutines)
			fmt.Printf("Changement ! nbPasParCycle (value): %v\n", paramExec.NbPasParCycle)
			fmt.Printf("Changement ! nbTriDansBubble (value): %v\n", paramExec.NbTriDansBubble)
			//envoi du signal de changement
			fileChange <- true
		}
	}
}

func Bubble(sem chan bool, nomOperation string, difOp float32) {
	// Bubble simule le comportement d'un microservice
	//  - Soit du traitement : simulé par un tri à bulle
	//  - et des appels à d'autre microservice, des BD, ... : simulé par des wait
	// il ne faut oublier que les microservices sont un ensemble de fonctions
	// et que ces fonction n'on pas le même comportement ...
	// simulé par nomOperation et et difOP

	//timer est un helper qui permet d'initialiser un timer de 1 à n mesures
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		server_duration_perc.WithLabelValues(ligneCmd.nomPod, ligneCmd.nomNode, ligneCmd.nomService, ligneCmd.nomService+ligneCmd.versionService, nomOperation).Observe(v)
		serveur_duration_histo.WithLabelValues(ligneCmd.nomPod, ligneCmd.nomNode, ligneCmd.nomService, ligneCmd.nomService+ligneCmd.versionService, nomOperation).Observe(v)
		// si contre les noms des dimensions :{   "Pod",            "NomNode",         "NomMicSrv",          "NomVerMicSrv",                        "MetMicSrv"}
	}))
	// lorsque bubble sera terminé enregistrement des mesures
	defer timer.ObserveDuration()
	// c'est tout ...

	for cp1 := 0; cp1 < paramExec.NbTriDansBubble; cp1++ {

		// 1 ko ...
		var aTrier [128]float64
		// initialisation du tableau dans la manière la moins favorable au trie au bulle
		// trier des valeurs entières, c'est trop facile
		for n := 0; n < 128; n++ {
			aTrier[n] = (128 - float64(n)) * math.Pi
		}

		//pour trier 128 valeurs --> 8256 test et permutations
		for i := 0; i < len(aTrier); i++ {
			for y := 0; y < len(aTrier)-1; y++ {
				if aTrier[y+1] < aTrier[y] {
					t := aTrier[y]
					aTrier[y] = aTrier[y+1]
					aTrier[y+1] = t
				}
			}
		}
		// pour décriminer la simulation des fonctions, multiplication
		// par le facteur passé en paramètre...
		t := float32(paramExec.DureeWaitDansBubble) * difOp

		// le temps dans le wait est réparti dans les bubble
		// pour simuler les temps actifs et les temps d'attentes ...
		t = t / float32(paramExec.NbTriDansBubble)
		time.Sleep(time.Duration(t) * time.Millisecond)
	}
	// mise à jour du compteur ...
	server_call_counter_count.WithLabelValues(ligneCmd.nomPod, ligneCmd.nomNode, ligneCmd.nomService, ligneCmd.nomService+ligneCmd.versionService, nomOperation).Inc()

	//C'est fini ... il faut le faire savoir !!!
	<-sem     //pour liberer un jeton pour permettre à une nouvelle goroutine d'être lancée
	wg.Done() //pour permettre la bonne fin de toutes les goroutines
}

func step(nbWorker int) {
	// Cette fonction gère le palier du nombre de worker passé en paramètre

	// ce tableau permet d'injecter des nom d'opération
	// pas utiliser dans le cadre de la démo, (il faut bien tenir dans l'heure ...)
	nameOp := []string{"Op01", "Op01"}
	multOp := []float32{1, 1} // variation du comportement des op {0.8, 1.2}

	// Gauge : enregistrement du nombre de worker en // pour ce pas
	server_call_gauge_count.WithLabelValues(ligneCmd.nomPod, ligneCmd.nomNode, ligneCmd.nomService, ligneCmd.nomService+ligneCmd.versionService).Set(float64(nbWorker))
	fmt.Printf(" Nombre workers en // :%v ", nbWorker)

	// Création  sémaphore de la taille pool de workers ...
	semaphore := make(chan bool, nbWorker)
	// Initialisation de l'heure de départ
	heureDepart := time.Now()
	// et de la permutation des noms de fonctions
	n := 0

	// Pour la durée configurée du plateau
	for time.Duration.Seconds(time.Since(heureDepart)) < float64(paramExec.DureeParPasSec) {

		// Un jeton en plus dans le sémaphore
		semaphore <- true // L'execution sera bloqué ICI lorsque le sémaphore sera plein
		// Pour attendre que tous les traitements soient terminés
		wg.Add(1)
		go Bubble(semaphore, nameOp[n], multOp[n])
		//fmt.Printf(" nom op: %v \n", nameOp[n])
		if n++; n > 1 {
			n = 0
		}
	}
	//bloque l'exécution pour attendre la bonne fin des Bubbles
	wg.Wait()
	// Tous les tir sont terminés
	durée := time.Since(heureDepart)
	fmt.Printf(" durée: %v \n", durée.Seconds())
}

func cycle(fileChange chan bool) {
	// cette fonction gère les différents paliers ascendants et descendants

	// pour toujours ;-)
	// lance une rampe linéaire de minPoolGoroutines à maxPoolGoroutines
	// puis revient à minPoolGoroutines !!!
	for {
		fmt.Printf("********************Début période****************************\n")
		// pour un palier ascendant
		for n := 0; n < paramExec.NbPasParCycle; n++ {
			fmt.Printf("Cycle ascendant pas :%v  ", n+1)
			// si il y a eu une modification du paramétrage (cas file change) break !!!
			// sinon on continue
			// Un des patterns sympa de go :-;
			select {
			case <-fileChange:
				break
			default:
				step(int(float64(paramExec.MaxPoolGoroutines-paramExec.MinPollGoroutines)/
					float64(paramExec.NbPasParCycle)*float64(n)) +
					paramExec.MinPollGoroutines)
			}
		}
		// pour le palier descendant
		for n := paramExec.NbPasParCycle; n >= 0; n-- {
			fmt.Printf("Cycle descendant pas :%v  ", n+1)
			select {
			case <-fileChange:
				break
			default:
				step(int(float64(paramExec.MaxPoolGoroutines-paramExec.MinPollGoroutines)/
					float64(paramExec.NbPasParCycle)*float64(n)) +
					paramExec.MinPollGoroutines)
			}
		}
		fmt.Printf("*************************************************************\n")
	}
}

func main() {

	// Récupération paramètres ligne de commande
	ptrIpPort := flag.String("ipPort", ":9911", "port exposition mesures pour prometheus")
	ptrNomPod := flag.String("nomPod", "Pod01", "identifiant du pod du microservice(pod)")
	ptrNomNode := flag.String("nomNode", "worker01", "nom du host")
	ptrNomService := flag.String("nomService", "Personne", "nom du service (Personne, Contrat, ...")
	PtrVersionService := flag.String("versionService", "1.0.0", "version du service ex:1.2.0")
	ptrNbProc := flag.Int("nbProc", 2, "nombre de coeurs à utiliser")
	ptrCfgPath := flag.String("cfgPath", "./", "chemin fichier de config")
	flag.Parse()

	// Maj de la structure LigneCmd
	ligneCmd.ipPort = *ptrIpPort
	ligneCmd.nomPod = *ptrNomPod
	ligneCmd.nomNode = *ptrNomNode
	ligneCmd.nomService = *ptrNomService
	ligneCmd.versionService = *PtrVersionService
	ligneCmd.nbProc = *ptrNbProc
	ligneCmd.cfgPath = *ptrCfgPath

	// Lecture de la configuration initiale
	paramExec = readFileConfig()

	// Rappel du contexte de l'exécution
	fmt.Printf("Version Go : %v\n", runtime.Version())
	fmt.Printf("build : V26/10-14:45\n")
	fmt.Printf("Param. ligne de commande, Port : %v\n", ligneCmd.ipPort)
	fmt.Printf("Param. ligne de commande, nom du pod : %v\n", ligneCmd.nomPod)
	fmt.Printf("Param. ligne de commande, nom du node : %v\n", ligneCmd.nomNode)

	fmt.Printf("Param. ligne de commande, nom du service : %v\n", ligneCmd.nomService)
	fmt.Printf("Param. ligne de commande, version du service : %v\n", ligneCmd.versionService)
	fmt.Printf("Param. ligne de commande, nombre de processeur : %v\n", ligneCmd.nbProc)
	fmt.Printf("Param. ligne de commande, path fichier config : %v\n", ligneCmd.cfgPath)
	fmt.Printf("initialisation dureeParPasSec (value): %v\n", paramExec.DureeParPasSec)
	fmt.Printf("initialisation dureeWaitDansBubble (value): %v\n", paramExec.DureeWaitDansBubble)
	fmt.Printf("initialisation maxPoolGoroutines (value): %v\n", paramExec.MaxPoolGoroutines)
	fmt.Printf("initialisation minPollGoroutines (value): %v\n", paramExec.MinPollGoroutines)
	fmt.Printf("initialisation nbPasParCycle (value): %v\n", paramExec.NbPasParCycle)
	fmt.Printf("initialisation nbTriDansBubble (value): %v\n", paramExec.NbTriDansBubble)

	// limitation du nombre de core
	runtime.GOMAXPROCS(ligneCmd.nbProc)

	// Indication d'un nouveau serveur actif (pourquoi celà ne sert à rien sur la condition de fin ?)
	server_actif_gauge.WithLabelValues(ligneCmd.nomPod, ligneCmd.nomNode, ligneCmd.nomService, ligneCmd.nomService+ligneCmd.versionService).Set(float64(1))
	defer server_actif_gauge.WithLabelValues(ligneCmd.nomPod, ligneCmd.nomNode, ligneCmd.nomService, ligneCmd.nomService+ligneCmd.versionService).Set(float64(0))

	//création du sémaphore de communication entre readconfig et cycle
	sm := make(chan bool)
	// lecture en tâche de fond du fichier de configuration
	go readconfig(sm)
	// lancement des tirs
	go cycle(sm)

	// exposition de :ipPort/metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(ligneCmd.ipPort, nil))
}
