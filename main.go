package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

type NetworkType string

const (
	roce NetworkType = "roce"
	ib   NetworkType = "ib"
)

var (
	namespace, configMapName string
	networkType              NetworkType = ib // default network type
	slurmibtopology_sh                   = "/usr/local/bin/slurmibtopology.sh"
)

func init() {
	if v := os.Getenv("NAMESPACE"); v != "" {
		namespace = v
	}
	if v := os.Getenv("CONFIG_MAP_NAME"); v != "" {
		configMapName = v
	}
	if v := os.Getenv("SLURMIBTOPOLOGY_SH"); v != "" {
		slurmibtopology_sh = v
	}
	if v := os.Getenv("NETWORK_TYPE"); v != "" {
		networkType = NetworkType(v)
	}
	logLevel := log.InfoLevel
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		if l, err := log.ParseLevel(v); err == nil {
			logLevel = l
		}
	}
	log.SetLevel(logLevel)
}

func GetIBConfig() ([]byte, error) {
	cmd := exec.Command("/bin/bash", "-c", slurmibtopology_sh)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).Fatalf("failed to run %s", slurmibtopology_sh)
		return nil, err
	}
	log.Tracef("output of %s: %s", slurmibtopology_sh, output)

	parsed := []byte{}
	s := bufio.NewScanner(bytes.NewReader(output))
	for s.Scan() {
		txt := s.Text()
		if strings.HasPrefix(txt, "SwitchName=") {
			// Append txt to a new bytes slice
			parsed = append(parsed, append(s.Bytes(), '\n')...)
		}
	}
	log.Tracef("parsed output: %s", parsed)

	return parsed, nil
}

func main() {
	// Validate the required environment variables
	if namespace == "" {
		log.Fatal("NAMESPACE environment variable is required")
	}
	if configMapName == "" {
		log.Fatal("CONFIG_MAP_NAME environment variable is required")
	}

	var content []byte
	switch networkType {
	case ib:
		ibConfig, err := GetIBConfig()
		if err != nil {
			log.WithError(err).Fatal("failed to get IB config")
		}
		content = ibConfig
	case roce:
		// TODO: implement RoCE config
		log.Fatal("RoCE config is not implemented yet")
	}

	// Use k8s client-go to update an existing ConfigMap
	config := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithError(err).Fatal("failed to create k8s client")
	}
	log.Info("Create k8s client successfully")
	cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		log.WithError(err).Fatalf("failed to get ConfigMap %s in namespace %s", configMapName, namespace)
	}
	cm.Data["topology.conf"] = base64.StdEncoding.EncodeToString(content)
	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		log.WithError(err).Fatalf("failed to update ConfigMap %s in namespace %s", configMapName, namespace)
	}

	log.Infof("successfully updated ConfigMap %s in namespace %s", configMapName, namespace)
}
