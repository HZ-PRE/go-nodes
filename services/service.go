package services

import (
	"sync"
	"time"

	"nodes/models"
	"nodes/models/vo"
	"nodes/repositories"
)

var (
	errUrlMap = make(map[string]uint)
	errIpMap  = make(map[string]uint)
	tjMap     = make(map[string]any)
	tjHMap    = make(map[string]any)
	errIpMu   sync.RWMutex
	tjMu      sync.RWMutex
	tjHMu     sync.RWMutex
)

type Service interface {
	ServerDictsDel(id string) error
	PostServerDicts(d models.ServerDict) error
	GetServerDictsByType(t uint8) ([]models.ServerDict, error)
	CreateServerLog(item models.ServerLog) error
	CreateServersNode(node models.ServerNode) error
	GetServerNodes() ([]vo.ServerNodeVo, error)
	GetServerNodesV1() ([]vo.ServerNodeVo, error)
	CreateServers(servers []models.Server, app string) error
	CreateServerStat(stats []models.ServerStat) error
	GetAppOnUser(start, end time.Time, flow uint) ([]vo.DaoVo, error)
	GetMaxOnUserByTime(h int) (*vo.DaoVo, error)
	GetOnUserByTime(h int, app string) ([]vo.DaoVo, error)
	GetServerStatsByDay(ip string) ([]vo.DaoVo, error)
	GetServerStatsByAppDay(app string) ([]vo.DaoVo, error)
	GetServerStats(start, end time.Time, flow, t uint) ([]vo.DaoVo, error)
	GetServerStatsInfo(ip string, start, end time.Time, flow uint) ([]vo.DaoVo, error)
	GetServerLog(app string) ([]models.ServerLog, error)
	TaskHour(before time.Time) error
	TaskDay()
	GetNodeNelNet() (map[string]uint, error)
	SelNodeNelNet(ip, prot string) (map[string]any, error)
	NodeNelNet() error
	ApiNelNet() error
	GetServersByPort() ([]models.Server, error)
	PostServerUrl(urls models.ServerUrl) error
	GetServerUrl() ([]models.ServerUrl, error)
	XrayCon(id string) (string, error)
	XrayInitCore(id string) (string, error)
	NodeExporterInit(id string) (string, error)
	LinuxCMD(id, cmd string) (string, error)
	GostCon(id, app string) (string, error)
	GetServersByApp(app string) ([]models.Server, error)
	GetServerNodesById(id string) ([]vo.ServerNodeVo, error)
	InitNodeMonitor() (string, error)
	ServerUrlDel(id string) error
	PostServerHost(hosts models.ServerHost) error
	GetServerHost() ([]models.ServerHost, error)
	GetServerHostByApp(app string) ([]models.ServerHost, error)
	ServerHostDel(id string) error
	PutServerHostByDomain(domain []string) error
	PostServerSupplierApi(api models.ServerSupplierApi) error
	GetServerSupplierApi() ([]models.ServerSupplierApi, error)
	GetServerSupplierApiBySupplier(supplier string, cdn int) ([]models.ServerSupplierApi, error)
	ServerSupplierApiDel(id int) error
	PostServerDomain(hosts models.ServerHost) error
}

type service struct {
	repo repositories.Repository
}

func NewService(repo repositories.Repository) Service {
	return &service{repo: repo}
}
func (s *service) TaskDay() {
	tjMu.Lock()
	tjMap = make(map[string]any)
	tjMu.Unlock()
	s.repo.ServerDel(time.Now().Add(-60 * time.Minute))
	s.repo.ServerStatTjYesterday()
	s.textServerHostBySslAt()
	s.uploadCDNDomainCert()
}
func (s *service) TaskHour(before time.Time) error {
	tjHMu.Lock()
	tjHMap = make(map[string]any)
	tjHMu.Unlock()
	if err := s.repo.ServerStatDeletes(before); err != nil {
		return err
	}
	if err := s.repo.ServerLogDeletes(before); err != nil {
		return err
	}
	if err := s.repo.UpdateServersNodeToInIp(); err != nil {
		return err
	}
	return s.repo.ServerStatTjHour()
}
