package repositories

import (
	"fmt"

	"nodes/database"
	"nodes/models"
	"nodes/models/vo"
)

func (r *repository) CreateServersNode(node models.ServerNode) error {
	var existing models.ServerNode
	result := database.DB.Select("id").Where("out_ip = ?", node.OutIP).First(&existing)
	if result.RowsAffected > 0 && existing.ID != node.ID {
		return fmt.Errorf("server node already exists: %s", node.OutIP)
	}

	return database.DB.Save(&node).Error
}
func (r *repository) UpdateServersNodeById(id string, node models.ServerNode) error {
	return database.DB.Model(&models.ServerNode{}).Where("id = ?", id).Updates(node).Error
}
func (r *repository) UpdateServersNodeToInIp() error {
	tx := database.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Exec(`UPDATE server_nodes AS n SET in_ip = sub.hosts,is_use = 1 FROM (SELECT n.id,STRING_AGG(DISTINCT s.host, ',' ORDER BY s.host) AS hosts FROM server_nodes AS n JOIN servers AS s ON n.out_ip = s.out_host WHERE n.city = ? GROUP BY n.id) AS sub WHERE n.id = sub.id`, "香港").Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Exec(`UPDATE server_nodes AS n SET is_use = 1 WHERE EXISTS (SELECT 1 FROM servers AS s WHERE s.out_host = n.out_ip AND s.updated_at >= NOW() - INTERVAL '1 hours') OR (zz_app is not null and zz_app!='' and updated_at >= NOW() - INTERVAL '1 hours')`).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
func (r *repository) GetServerNodes() ([]vo.ServerNodeVo, error) {
	rows := make([]vo.ServerNodeVo, 0)
	query := "SELECT n.zz_app,n.id,n.in_ip,n.out_ip,n.note,n.city,n.in_ip_port,n.in_ip_pwd,n.in_ip_user,n.out_ip_port,n.out_ip_pwd,n.out_ip_user,n.dw,n.created_at,n.updated_at,n.device,n.is_use,n.is_xray,STRING_AGG(DISTINCT concat(s.app, s.name), ',') AS names FROM server_nodes n LEFT JOIN servers s ON n.out_ip = s.out_host GROUP BY n.zz_app,n.id,n.in_ip,n.out_ip,n.note,n.city,n.in_ip_port,n.in_ip_pwd,n.in_ip_user,n.out_ip_port,n.out_ip_pwd,n.out_ip_user,n.dw,n.created_at,n.updated_at,n.device,n.is_use,n.is_xray ORDER BY updated_at DESC"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerNodesV1() ([]vo.ServerNodeVo, error) {
	rows := make([]vo.ServerNodeVo, 0)
	query := "SELECT n.in_ip,n.out_ip,n.note,n.city,n.dw,n.device,n.created_at,COALESCE(STRING_AGG(DISTINCT concat(s.app, s.name), ',') FILTER (WHERE n.out_ip = s.out_host),n.note) AS names,STRING_AGG(DISTINCT s.app, ',') as zz_app FROM server_nodes n INNER JOIN servers s ON (n.out_ip = s.out_host OR s.host = ANY(string_to_array(n.in_ip, ','))) GROUP BY n.in_ip,n.out_ip,n.note,n.city,n.dw,n.device,n.created_at ORDER BY n.created_at DESC"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerNodesById(id string) ([]vo.ServerNodeVo, error) {
	rows := make([]vo.ServerNodeVo, 0)
	query := "SELECT s.host as in_ip,n.out_ip,n.in_ip_port,n.in_ip_pwd,n.in_ip_user,n.out_ip_port,n.out_ip_pwd,n.out_ip_user,s.node_id,s.app as zz_app,s.method,s.name as names FROM server_nodes n left  JOIN servers s ON n.out_ip = s.out_host AND (s.parent_id is NULL or s.parent_id =0) and s.updated_at >= now() - INTERVAL '2 minute' WHERE s.node_id is not null AND n.id = ?"
	err := database.DB.Raw(query, id).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerNodesByIdV1(id string) (models.ServerNode, error) {
	var node models.ServerNode
	err := database.DB.Where("id = ?", id).First(&node).Error
	return node, err
}
func (r *repository) GetAllUseServerNode() ([]models.ServerNode, error) {
	rows := make([]models.ServerNode, 0)
	query := "SELECT n.id,n.out_ip,n.city FROM server_nodes n INNER JOIN servers s ON (n.out_ip = s.out_host OR s.host = ANY(string_to_array(n.in_ip, ','))) GROUP BY n.id,n.out_ip,n.city"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
