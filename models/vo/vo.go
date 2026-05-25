package vo

import (
	"nodes/models"
	"time"
)

type DaoVo struct {
	StartTime time.Time `json:"start_time"`
	OnLinkSum int64     `json:"on_link_sum"`
	Apps      string    `json:"apps"`
	OutIP     string    `json:"out_ip"`
	InIP      string    `json:"in_ip"`
}

type ServerNodeVo struct {
	models.ServerNode `gorm:"embedded"`
	Names             string `json:"names"`
	Method            string `json:"method"`
	NodeID            uint   `json:"node_id"`
	OnLinkSum         int64  `json:"on_link_sum"`
}
type ServerHostVo struct {
	models.ServerHost `gorm:"embedded"`
	Key               string `json:"key"`
	Secret            string `json:"secret"`
}
type ServerStatVo struct {
	App  string `json:"app"`
	IP   string `json:"ip"`
	Prot string `json:"prot"`
	H    int    `json:"h"`
}
