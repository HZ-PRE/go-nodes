package repositories

import (
	"time"

	"nodes/models"
	"nodes/models/vo"
)

const deleteBatchSize = 5000

type Repository interface {
	ServerDictsDel(id string) error
	PostServerDicts(d models.ServerDict) error
	CreateServersNode(node models.ServerNode) error
	CreateServers(servers []models.Server) error
	CreateServerStat(stats []models.ServerStat) error
	CreateServerLog(item models.ServerLog) error
	GetServerNodes() ([]vo.ServerNodeVo, error)
	GetServerNodesV1() ([]vo.ServerNodeVo, error)
	GetServerStatsByDay(ip string) ([]vo.DaoVo, error)
	GetServerStatsByAppDay(app string) ([]vo.DaoVo, error)
	GetServerStats(start, end time.Time, flow, t uint) ([]vo.DaoVo, error)
	GetMaxOnUserByTime(h int) (*vo.DaoVo, error)
	GetOnUserByTime(h int, app string) ([]vo.DaoVo, error)
	GetAppOnUser(start, end time.Time, flow uint) ([]vo.DaoVo, error)
	GetServerStatsInfo(ip string, start, end time.Time, flow uint) ([]vo.DaoVo, error)
	GetServers() ([]models.Server, error)
	GetServerLog(app string) ([]models.ServerLog, error)
	ServerStatDeletes(before time.Time) error
	ServerLogDeletes(before time.Time) error
	PostServerUrl(urls models.ServerUrl) error
	GetServerUrl() ([]models.ServerUrl, error)
	GetServerNodesById(id string) ([]vo.ServerNodeVo, error)
	GetServerNodesByIdV1(id string) (models.ServerNode, error)
	GetServerDictsByType(t uint8) ([]models.ServerDict, error)
	ServerDeletes(time time.Time, app string) error
	GetServersByApp(app string) ([]models.Server, error)
	GetServersByPort() ([]models.Server, error)
	UpdateServersNodeById(id string, node models.ServerNode) error
	ServerDel(time time.Time) error
	GetAllUseServerNode() ([]models.ServerNode, error)
	ServerUrlDel(id string) error
	UpdateServersNodeToInIp() error
	ServerStatTjYesterday() error
	ServerStatTjHour() error
	PostServerHost(hosts models.ServerHost) error
	GetServerHost() ([]models.ServerHost, error)
	GetServerHostByApp(app string) ([]models.ServerHost, error)
	ServerHostDel(id string) error
	PutServerHostByDomain(domain []string) error
	PostServerSupplierApi(api models.ServerSupplierApi) error
	GetServerSupplierApi() ([]models.ServerSupplierApi, error)
	GetServerSupplierApiBySupplier(supplier string, cdn int) ([]models.ServerSupplierApi, error)
	ServerSupplierApiDel(id int) error
	GetServerSupplierApiById(id int) (models.ServerSupplierApi, error)
	GetServerSupplierApiByIdV1(id int) (models.ServerSupplierApi, error)
	GetServerHostByParentId(parentIds []int) ([]vo.ServerHostVo, error)
	GetServerHostBySslAt(day int) ([]vo.ServerHostVo, error)
	PutServerHostById(id int, ssl_at time.Time) error
}

type repository struct{}

func NewRepository() Repository {
	return &repository{}
}
