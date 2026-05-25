package repositories

import (
	"nodes/database"
	"nodes/models"
	"nodes/models/vo"
	"time"
)

func (r *repository) PostServerHost(hosts models.ServerHost) error {
	result := database.DB.Exec(
		"UPDATE server_hosts SET status = $1 WHERE parent_id = $2",
		hosts.Status, hosts.ID,
	)
	if result.Error != nil {
		return result.Error
	}
	return database.DB.Save(&hosts).Error
}
func (r *repository) PutServerHostByDomain(domain []string) error {
	result := database.DB.
		Table("server_hosts").
		Where("domain IN ?", domain).
		Update("is_use", 1)
	if result.Error != nil {
		return result.Error
	}
	return database.DB.Exec(
		"UPDATE server_hosts SET is_use = 1 WHERE id in (SELECT parent_id FROM server_hosts WHERE domain in ?)",
		domain,
	).Error
}
func (r *repository) PutServerHostById(ids int, ssl_at time.Time) error {
	return database.DB.
		Table("server_hosts").
		Where("id = ?", ids).
		Update("ssl_at", ssl_at).Error
}
func (r *repository) GetServerHost() ([]models.ServerHost, error) {
	rows := make([]models.ServerHost, 0)
	query := "SELECT * FROM server_hosts ORDER BY id DESC"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerHostByApp(app string) ([]models.ServerHost, error) {
	rows := make([]models.ServerHost, 0)
	query := "SELECT domain FROM server_hosts WHERE status=0 AND app = ?"
	err := database.DB.Raw(query, app).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerHostBySslAt(day int) ([]vo.ServerHostVo, error) {
	rows := make([]vo.ServerHostVo, 0)
	query := "SELECT h.id,h.scope,h.domain,h.parent_id,h.origin_domain,s.key,s.secret,s.supplier_account,s.supplier FROM server_hosts as h INNER JOIN server_supplier_apis as s on h.supplier_id=s.id and h.ssl_at<NOW() + INTERVAL '? day' and h.ssl_at>NOW() - INTERVAL '1 day' and h.parent_id>0 and h.is_self=1 and (h.scope =0 or (h.scope>0 and h.beian=1))"
	err := database.DB.Raw(query, day).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerHostByParentId(parentIds []int) ([]vo.ServerHostVo, error) {
	rows := make([]vo.ServerHostVo, 0)
	query := "SELECT h.id,h.scope,h.domain,h.parent_id,h.origin_domain,s.key,s.secret,s.supplier_account,s.supplier FROM server_hosts as h INNER JOIN server_supplier_apis as s on h.supplier_id=s.id WHERE h.parent_id IN ?"
	err := database.DB.Raw(query, parentIds).Scan(&rows).Error
	return rows, err
}
func (r *repository) ServerHostDel(id string) error {
	return database.DB.Exec("DELETE FROM server_hosts WHERE id = $1 or parent_id = $1", id).Error
}
func (r *repository) PostServerUrl(urls models.ServerUrl) error {
	return database.DB.Save(&urls).Error
}
func (r *repository) GetServerUrl() ([]models.ServerUrl, error) {
	rows := make([]models.ServerUrl, 0)
	query := "SELECT * FROM server_urls ORDER BY id DESC"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
func (r *repository) ServerUrlDel(id string) error {
	return database.DB.Exec("DELETE FROM server_urls WHERE id = $1", id).Error
}
func (r *repository) PostServerSupplierApi(api models.ServerSupplierApi) error {
	return database.DB.Save(&api).Error
}
func (r *repository) GetServerSupplierApi() ([]models.ServerSupplierApi, error) {
	rows := make([]models.ServerSupplierApi, 0)
	query := "SELECT * FROM server_supplier_apis ORDER BY id DESC"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerSupplierApiById(id int) (models.ServerSupplierApi, error) {
	var api models.ServerSupplierApi
	query := "SELECT * FROM server_supplier_apis WHERE id = ?"
	err := database.DB.Raw(query, id).Scan(&api).Error
	return api, err
}
func (r *repository) GetServerSupplierApiByIdV1(id int) (models.ServerSupplierApi, error) {
	var api models.ServerSupplierApi
	query := "SELECT * FROM server_supplier_apis WHERE id in (SELECT supplier_id FROM server_hosts WHERE id=?)"
	err := database.DB.Raw(query, id).Scan(&api).Error
	return api, err
}
func (r *repository) GetServerSupplierApiBySupplier(supplier string, cdn int) ([]models.ServerSupplierApi, error) {
	rows := make([]models.ServerSupplierApi, 0)
	query := "SELECT * FROM server_supplier_apis WHERE status=0 AND supplier = ? ORDER BY id ASC"
	if cdn == 1 {
		query = "SELECT * FROM server_supplier_apis WHERE status=0 AND supplier = ? AND cdn=1 ORDER BY id ASC"
	}
	err := database.DB.Raw(query, supplier).Scan(&rows).Error
	return rows, err
}
func (r *repository) ServerSupplierApiDel(id int) error {
	return database.DB.Table("server_supplier_apis").Where("id = ?", id).Update("status", 1).Error
}
func (r *repository) GetServerDictsByType(t uint8) ([]models.ServerDict, error) {
	rows := make([]models.ServerDict, 0)
	query := "SELECT * FROM server_dicts WHERE type = ?"
	err := database.DB.Raw(query, t).Scan(&rows).Error
	return rows, err
}
func (r *repository) PostServerDicts(d models.ServerDict) error {
	return database.DB.Save(&d).Error
}
func (r *repository) ServerDictsDel(id string) error {
	return database.DB.Exec("DELETE FROM server_urls WHERE id = $1", id).Error
}
