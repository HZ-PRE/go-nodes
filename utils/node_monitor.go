package utils

import (
	"fmt"
	"log"
	"net/http"
	"nodes/models"
	"os"
	"path/filepath"
	"sync"

	"github.com/goccy/go-yaml"
)

type Target struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

var mu sync.Mutex

func NodeMonitor(nodes []models.ServerNode) error {
	mu.Lock()
	defer mu.Unlock()
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes, skip")
	}
	var data []Target
	for _, n := range nodes {
		data = append(data, Target{
			Targets: []string{fmt.Sprintf("%s:%d", n.OutIP, 13240)},
			Labels: map[string]string{
				"region":  n.City,
				"node_id": n.ID,
				"ip":      n.OutIP,
			},
		})
	}
	dir := "prometheus"
	finalPath := filepath.Join(dir, "nodes.yml")
	tmpPath := finalPath + ".tmp"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, b, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return err
	}
	resp, err := http.Post("http://127.0.0.1:9090/-/reload", "application/json", nil)
	if err != nil {
		log.Println("reload failed:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Println("reload failed, status:", resp.Status)
		return err
	}
	log.Println("NodeMonitor updated successfully")
	return nil
}
