package repositories

import (
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nodes/database"
	"nodes/models"
	"nodes/models/vo"
)

func (r *repository) CreateServers(servers []models.Server) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&servers).Error
	})

}
func (r *repository) ServerDeletes(time time.Time, app string) error {
	for {
		result := database.DB.Exec("DELETE FROM servers WHERE updated_at < $1 AND app = $2", time, app)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			break
		}
	}
	return nil
}

func (r *repository) CreateServerStat(stats []models.ServerStat) error {
	return database.DB.CreateInBatches(stats, 64).Error
}

func (r *repository) CreateServerLog(item models.ServerLog) error {
	return database.DB.Create(&item).Error
}

func (r *repository) GetServerStatsByDay(ip string) ([]vo.DaoVo, error) {
	rows := make([]vo.DaoVo, 0)
	query := "SELECT date_trunc('day', created_at) AS start_time, COUNT(DISTINCT (user_id, app)) AS on_link_sum, array_agg(DISTINCT app) AS apps FROM server_stats WHERE node_ip = ? AND created_at >= NOW() - INTERVAL '7 days' AND created_at <= NOW() GROUP BY start_time ORDER BY start_time DESC"
	err := database.DB.Raw(query, ip).Scan(&rows).Error
	return rows, err
}

func (r *repository) GetServerStatsByAppDay(app string) ([]vo.DaoVo, error) {
	rows := make([]vo.DaoVo, 0)
	query := "SELECT date_trunc('day', times) as start_time, sum(CAST(val AS INTEGER)) AS on_link_sum FROM server_bi_tables WHERE times >= NOW() - INTERVAL '7 days' AND times <= NOW() AND type='1'"
	args := []any{}
	if app != "" {
		query += " AND keys = ?"
		args = append(args, app)
	}
	query += " GROUP BY start_time ORDER BY start_time"
	err := database.DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (r *repository) GetServerStats(start, end time.Time, flow, t uint) ([]vo.DaoVo, error) {
	rows := make([]vo.DaoVo, 0)
	node_query := "e.node_ip"
	if t == 1 {
		node_query = "CASE WHEN (s.tags IS NULL OR s.tags='') THEN e.node_ip ELSE s.tags END"
	}
	query := "SELECT " + node_query + " AS out_ip, COUNT(DISTINCT (e.user_id, e.app)) AS on_link_sum, array_agg(DISTINCT (s.app || s.name)) AS apps, array_agg(DISTINCT s.host) AS in_ip FROM server_stats AS e INNER JOIN servers AS s ON s.app = e.app AND (s.node_id = e.node_id OR s.parent_id = e.node_id) WHERE e.created_at >= ? AND e.created_at <= ?"
	args := []any{start, end}
	if flow != 0 {
		query += " AND (u + d) > ?"
		args = append(args, flow*8)
	}
	query += " GROUP BY out_ip ORDER BY on_link_sum DESC"
	err := database.DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (r *repository) GetMaxOnUserByTime(h int) (*vo.DaoVo, error) {
	out := &vo.DaoVo{}
	query := "SELECT date_trunc('minute', times) AS start_time, sum(CAST(val AS INTEGER)) AS on_link_sum FROM server_bi_tables WHERE type='2' and times >=? AND times <= NOW() GROUP BY start_time ORDER BY on_link_sum DESC LIMIT 1"
	start := time.Now().Add(-time.Duration(h) * time.Hour)
	err := database.DB.Raw(query, start).Scan(out).Error
	return out, err
}

func (r *repository) GetOnUserByTime(h int, app string) ([]vo.DaoVo, error) {
	rows := make([]vo.DaoVo, 0)
	var query string
	var args []any

	if h > 0 && h != 24 {
		query = "SELECT date_trunc('minute', times) AS start_time, sum(CAST(val AS INTEGER)) AS on_link_sum FROM server_bi_tables WHERE type='2' and times >= ? AND times <= NOW()"
		args = append(args, time.Now().Add(-time.Duration(h)*time.Hour))
	} else {
		query = "SELECT date_trunc('minute', times) AS start_time, sum(CAST(val AS INTEGER)) AS on_link_sum FROM server_bi_tables WHERE type='2' and times >= NOW() - INTERVAL '24 hours' AND times <= NOW()"
	}
	if app != "" {
		query += " AND keys = ?"
		args = append(args, app)
	}
	query += " GROUP BY start_time ORDER BY start_time"
	err := database.DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (r *repository) GetAppOnUser(start, end time.Time, flow uint) ([]vo.DaoVo, error) {
	rows := make([]vo.DaoVo, 0)
	query := "SELECT app AS apps, COUNT(DISTINCT (user_id, app)) AS on_link_sum FROM server_stats WHERE created_at >= ? AND created_at <= ?"
	args := []any{start, end}
	if flow != 0 {
		query += " AND (u + d) > ?"
		args = append(args, flow*8)
	}
	query += " GROUP BY app ORDER BY on_link_sum DESC"
	err := database.DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (r *repository) GetServerStatsInfo(ip string, start, end time.Time, flow uint) ([]vo.DaoVo, error) {
	rows := make([]vo.DaoVo, 0)
	query := "SELECT COUNT(DISTINCT (e.user_id, e.app)) AS on_link_sum, concat(s.app, s.name) AS apps, s.host AS in_ip FROM server_stats AS e INNER JOIN servers AS s ON s.app = e.app AND s.node_id = e.node_id WHERE e.created_at >= ? AND e.created_at <= ? AND e.node_ip = ?"
	args := []any{start, end, ip}
	if flow != 0 {
		query += " AND (e.u+e.d) > ?"
		args = append(args, flow*8)
	}
	query += " GROUP BY apps, in_ip ORDER BY on_link_sum DESC"
	err := database.DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (r *repository) GetServers() ([]models.Server, error) {
	rows := make([]models.Server, 0)
	err := database.DB.Select("name, host, port,app").Where("name NOT LIKE ?", "%-加速").Find(&rows).Error
	return rows, err
}
func (r *repository) GetServersByApp(app string) ([]models.Server, error) {
	rows := make([]models.Server, 0)
	err := database.DB.Select("node_id,port,server_port,out_host,name").Where("app = ? and name not like '%香港%' and name not like '%-加速' AND (parent_id is NULL or parent_id =0) ", app).Find(&rows).Error
	return rows, err
}

func (r *repository) GetServersByPort() ([]models.Server, error) {
	rows := make([]models.Server, 0)
	query := "SELECT app,name,port from servers WHERE port in (SELECT port FROM servers WHERE (parent_id is NULL or parent_id =0) and name not like '%香港%' and name not like '%-加速%' and updated_at >= now() - INTERVAL '2 minute' GROUP BY port HAVING count(1)>1) and (parent_id is NULL or parent_id =0) and updated_at >= now() - INTERVAL '2 minute' and name not like '%香港%' and name not like '%-加速%'"
	err := database.DB.Raw(query).Scan(&rows).Error
	return rows, err
}
func (r *repository) GetServerLog(app string) ([]models.ServerLog, error) {
	rows := make([]models.ServerLog, 0)
	q := database.DB.Order("created_at DESC")
	if app != "" {
		q = q.Where("app = ?", app)
	}
	err := q.Limit(200).Find(&rows).Error
	return rows, err
}

func (r *repository) ServerStatDeletes(before time.Time) error {
	for {
		result := database.DB.Exec(
			"DELETE FROM server_stats WHERE id IN (SELECT id FROM server_stats WHERE created_at < $1 LIMIT $2)",
			before, deleteBatchSize,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			break
		}
	}
	return nil
}

func (r *repository) ServerLogDeletes(before time.Time) error {
	for {
		result := database.DB.Exec(
			"DELETE FROM server_logs WHERE id IN (SELECT id FROM server_logs WHERE created_at < $1 LIMIT $2)",
			before, deleteBatchSize,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			break
		}
	}
	return nil
}

func (r *repository) ServerDel(time time.Time) error {
	return database.DB.Exec("DELETE FROM servers WHERE updated_at < $1", time).Error
}
func (r *repository) ServerStatTjYesterday() error {
	start := time.Now()
	err := database.DB.Exec("CALL public.insert_server_bi_stats_yesterday()").Error
	log.Printf("ServerStatTjYesterday cost: %v", time.Since(start))
	return err
}
func (r *repository) ServerStatTjHour() error {
	start := time.Now()
	err := database.DB.Exec("CALL public.insert_server_bi_stats_hour()").Error
	log.Printf("ServerStatTjHour cost: %v", time.Since(start))
	return err
}
