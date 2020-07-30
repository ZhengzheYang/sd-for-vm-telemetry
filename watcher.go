// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"

	"istio.io/client-go/pkg/apis/networking/v1beta1"
	"istio.io/client-go/pkg/clientset/versioned"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
)

type Watcher struct {
	istioClient          *versioned.Clientset
	k8sClient            *kubernetes.Clientset
	namespace            string
	Watch                watch.Interface
	requiredTerminations sync.WaitGroup
}

func NewWatcher(restConfig *rest.Config) *Watcher {
	// istio client
	ic, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create istio client: %s", err)
	}

	// k8s client
	k8sClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %s", err)

	}
	namespace := ""		// get workload from all namespaces
	watchWLE, err := ic.NetworkingV1beta1().WorkloadEntries(namespace).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to get Workload Entry watch: %v", err)
	}
	w := &Watcher{
		istioClient: ic,
		k8sClient:   k8sClientSet,
		namespace:   namespace,
		Watch:       watchWLE,
	}
	log.Println("workload entry watcher created")
	return w
}

// Start the workload entry watcher. It could be stopped with keyboard interrupt
func (w *Watcher) Start(stop <-chan struct{}) {
	go func() {
		w.requiredTerminations.Add(1)
		for event := range w.Watch.ResultChan() {
			fileSDConfig, err := getOrCreatePromSDConfigMap(w.k8sClient)
			if err != nil {
				log.Printf("get or create config map failed: %v\n", err)
			}
			wle, ok := event.Object.(*v1beta1.WorkloadEntry)
			if !ok {
				log.Print("unexpected type")
			}
			wleName := createNameFromAddr(wle.Spec.Address)

			switch event.Type {
			case watch.Deleted:
				log.Printf("handle deleted workload %s", wle.Spec.Address)
				if fileSDConfig.Data != nil {
					delete(fileSDConfig.Data, fmt.Sprintf("%s.yaml", wleName))
				}
			default: // add or update
				log.Printf("handle update workload %s", wle.Spec.Address)
				if fileSDConfig.Data == nil {
					fileSDConfig.Data = make(map[string]string)
				}
				staticConfig := `
- targets:
  - %s
`
				fileSDConfig.Data[fmt.Sprintf("%s.yaml", wleName)] = fmt.Sprintf(staticConfig, wle.Spec.Address)
			}
			if err := updatePromSDConfigMap(w.k8sClient, fileSDConfig); err != nil {
				log.Printf("update config map failed: %v\n", err)
			}
		}
		w.requiredTerminations.Done()
	}()
	w.waitForShutdown(stop)
}

func (w *Watcher) waitForShutdown(stop <-chan struct{}) {
	go func() {
		<-stop
		w.Watch.Stop()
		w.requiredTerminations.Wait()
	}()
}

func createNameFromAddr(ip string) string {
	return strings.ReplaceAll(ip, ".", "-")
}

func getOrCreatePromSDConfigMap(client *kubernetes.Clientset) (*v1.ConfigMap, error) {
	configMap, err := client.CoreV1().ConfigMaps("istio-system").
		Get(context.TODO(), "file-sd-config", metav1.GetOptions{})
	if err == nil {
		// config map exists, return directly
		return configMap, nil
	}
	cfg := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "file-sd-config",
		},
		Data: make(map[string]string),
	}
	if configMap, err = client.CoreV1().ConfigMaps("istio-system").Create(context.TODO(), cfg,
		metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	return configMap, nil
}

func updatePromSDConfigMap(client *kubernetes.Clientset, fileSDConfig *v1.ConfigMap) error {
	// Write the update config map back to cluster
	if _, err := client.CoreV1().ConfigMaps("istio-system").Update(context.TODO(), fileSDConfig,
		metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
