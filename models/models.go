package models

import (
	"encoding/json"
	"time"
)

type Server struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `json:"name"`
	App        string    `json:"app"`
	Host       string    `json:"host"`
	OutHost    string    `json:"out_host"`
	NodeID     uint      `json:"node_id"`
	ParentID   uint      `json:"parent_id"`
	Tags       string    `json:"tags"`
	Port       uint      `json:"port"`
	ServerPort uint      `json:"server_port"`
	Method     string    `json:"method"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ServerNode struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	InIP      string    `json:"in_ip"`
	InIPPwd   string    `json:"in_ip_pwd"`
	InIPUser  string    `json:"in_ip_user"`
	OutIP     string    `json:"out_ip"`
	OutIPUser string    `json:"out_ip_user"`
	OutIPPwd  string    `json:"out_ip_pwd"`
	Dw        int       `json:"dw"`
	IsXray    int       `json:"is_xray"`
	OutIPPort int       `json:"out_ip_port"`
	InIPPort  int       `json:"in_ip_port"`
	Note      string    `json:"note"`
	ZzApp     string    `json:"zz_app"`
	Device    string    `json:"device"`
	City      string    `json:"city"`
	IsUse     int       `json:"is_use"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerStat struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	App       string    `json:"app"`
	NodeID    string    `json:"node_id"`
	NodeIP    string    `json:"node_ip"`
	UserID    string    `json:"user_id"`
	Method    string    `json:"method"`
	Rate      string    `json:"rate"`
	U         int64     `json:"u"`
	D         int64     `json:"d"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerStatRequest struct {
	NodeIP string             `json:"node_ip"`
	NodeID json.Number        `json:"node_id"`
	App    string             `json:"app"`
	Method string             `json:"method"`
	Rate   string             `json:"rate"`
	Users  map[string][]int64 `json:"users"`
}
type ServerUrl struct {
	ID   string `json:"id"`
	Url  string `json:"url"`
	Note string `json:"note"`
}
type ServerHost struct {
	ID              int       `json:"id"`
	Domain          string    `json:"domain"`
	OriginDomain    string    `json:"origin_domain"`
	Supplier        string    `json:"supplier"`
	ParentID        int       `json:"parent_id"`
	SupplierAccount string    `json:"supplier_account"`
	Status          uint8     `json:"status"`
	IsUse           uint8     `json:"is_use"`
	Note            string    `json:"note"`
	App             string    `json:"app"`
	CreatedAt       time.Time `json:"created_at"`
	SslAt           time.Time `json:"ssl_at"`
	SupplierId      int       `json:"supplier_id"`
	IsSelf          uint8     `json:"is_self"`
	Scope           int       `json:"scope"`
	Beian           int       `json:"beian"`
}
type ServerSupplierApi struct {
	ID              int    `json:"id"`
	Key             string `json:"key"`
	Secret          string `json:"secret"`
	Supplier        string `json:"supplier"`
	SupplierAccount string `json:"supplier_account"`
	Note            string `json:"note"`
	Status          uint8  `json:"status"`
	Scope           int    `json:"scope"`
	Cdn             int    `json:"cdn"`
}
type ServerDict struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Val  string `json:"val"`
	Type uint8  `json:"type"`
	Note string `json:"note"`
}

func (r *ServerStatRequest) ToServerStats() []ServerStat {
	stats := make([]ServerStat, 0, len(r.Users))
	for userID, traffic := range r.Users {
		var u, d int64
		if len(traffic) > 0 {
			u = traffic[0]
		}
		if len(traffic) > 1 {
			d = traffic[1]
		}
		stats = append(stats, ServerStat{
			App:    r.App,
			NodeID: r.NodeID.String(),
			NodeIP: r.NodeIP,
			UserID: userID,
			Method: r.Method,
			Rate:   r.Rate,
			U:      u,
			D:      d,
		})
	}
	return stats
}

type ServerLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	App       string    `json:"app"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
