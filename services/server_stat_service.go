package services

import (
	"fmt"
	"strconv"
	"time"

	"nodes/models"
	"nodes/models/vo"
	"nodes/utils"
)

func (s *service) GetServerNodesById(id string) ([]vo.ServerNodeVo, error) {
	return s.repo.GetServerNodesById(id)
}
func (s *service) GetServersByApp(app string) ([]models.Server, error) {
	return s.repo.GetServersByApp(app)
}
func (s *service) CreateServerLog(item models.ServerLog) error {
	if utils.Node != nil {
		item.ID = uint(utils.Node.Generate().Int64())
	}
	return s.repo.CreateServerLog(item)
}

func (s *service) CreateServersNode(node models.ServerNode) error {
	now := time.Now()
	if node.ID == "" && utils.Node != nil {
		node.ID = utils.Node.Generate().String()
		node.CreatedAt = now
	}
	node.UpdatedAt = now
	return s.repo.CreateServersNode(node)
}

func (s *service) GetServerNodes() ([]vo.ServerNodeVo, error) {
	return s.repo.GetServerNodes()
}
func (s *service) GetServerNodesV1() ([]vo.ServerNodeVo, error) {
	return s.repo.GetServerNodesV1()
}
func (s *service) CreateServers(servers []models.Server, app string) error {
	now := time.Now()
	for i := range servers {
		servers[i].NodeID = servers[i].ID
		servers[i].ID = uint(utils.ParseBase36Fast2(fmt.Sprintf("%s%d", app, servers[i].ID)))
		servers[i].App = app
		servers[i].UpdatedAt = now
	}
	err := s.repo.CreateServers(servers)
	if err == nil {
		err = s.repo.ServerDeletes(now, app)
	}
	return err
}

func (s *service) CreateServerStat(stats []models.ServerStat) error {
	now := time.Now()
	for i := range stats {
		if utils.Node != nil {
			stats[i].ID = uint(utils.Node.Generate().Int64())
		}
		stats[i].CreatedAt = now
		stats[i].UpdatedAt = now
	}
	return s.repo.CreateServerStat(stats)
}

func (s *service) GetAppOnUser(start, end time.Time, flow uint) ([]vo.DaoVo, error) {
	return s.repo.GetAppOnUser(start, end, flow)
}

func (s *service) GetMaxOnUserByTime(h int) (*vo.DaoVo, error) {
	n := fmt.Sprintf("%s_%d", "GetMaxOnUserByTime", h)
	tjHMu.RLock()
	cached, ok := tjHMap[n]
	tjHMu.RUnlock()
	if ok {
		if out, castOK := cached.(*vo.DaoVo); castOK {
			return out, nil
		}
	}
	ret, err := s.repo.GetMaxOnUserByTime(h)
	if err == nil {
		tjHMu.Lock()
		tjHMap[n] = ret
		tjHMu.Unlock()
	}
	return ret, err
}

func (s *service) GetOnUserByTime(h int, app string) ([]vo.DaoVo, error) {
	n := fmt.Sprintf("%s_%s_%d", app, "GetOnUserByTime", h)
	tjHMu.RLock()
	cached, ok := tjHMap[n]
	tjHMu.RUnlock()
	if ok {
		if out, castOK := cached.([]vo.DaoVo); castOK {
			return out, nil
		}
	}
	ret, err := s.repo.GetOnUserByTime(h, app)
	if err == nil {
		tjHMu.Lock()
		tjHMap[n] = ret
		tjHMu.Unlock()
	}
	return ret, err

}

func (s *service) GetServerStatsByDay(ip string) ([]vo.DaoVo, error) {
	n := fmt.Sprintf("%s_%s", ip, "GetServerStatsByDay")
	tjMu.RLock()
	cached, ok := tjMap[n]
	tjMu.RUnlock()
	if ok {
		if out, castOK := cached.([]vo.DaoVo); castOK {
			return out, nil
		}
	}
	ret, err := s.repo.GetServerStatsByDay(ip)
	if err == nil {
		tjMu.Lock()
		tjMap[n] = ret
		tjMu.Unlock()
	}
	return ret, err
}

func (s *service) GetServerStatsByAppDay(app string) ([]vo.DaoVo, error) {
	n := fmt.Sprintf("%s_%s", app, "GetServerStatsByAppDay")
	tjMu.RLock()
	cached, ok := tjMap[n]
	tjMu.RUnlock()
	if ok {
		if out, castOK := cached.([]vo.DaoVo); castOK {
			return out, nil
		}
	}
	ret, err := s.repo.GetServerStatsByAppDay(app)
	if err == nil {
	}
	return ret, err
}

func (s *service) GetServerStats(start, end time.Time, flow, t uint) ([]vo.DaoVo, error) {
	return s.repo.GetServerStats(start, end, flow, t)
}

func (s *service) GetServerStatsInfo(ip string, start, end time.Time, flow uint) ([]vo.DaoVo, error) {
	return s.repo.GetServerStatsInfo(ip, start, end, flow)
}

func (s *service) GetServerLog(app string) ([]models.ServerLog, error) {
	return s.repo.GetServerLog(app)
}

func (s *service) GetServersByPort() ([]models.Server, error) {
	return s.repo.GetServersByPort()
}
func (s *service) GetNodeNelNet() (map[string]uint, error) {
	errIpMu.RLock()
	snapshot := make(map[string]uint, len(errIpMap))
	for k, v := range errIpMap {
		snapshot[k] = v
	}
	errIpMu.RUnlock()
	return snapshot, nil
}

func (s *service) SelNodeNelNet(ip, prot string) (map[string]any, error) {
	result, err := utils.IpNelNet(ip, prot)
	if err != nil {
		result = YCIpBC(map[string]string{fmt.Sprintf("%s:%s", ip, prot): ""}, false)
		if !result {
			err = fmt.Errorf("probe failed for %s:%s", ip, prot)
		}
	}
	return map[string]any{"ip": ip, "prot": prot, "result": result}, err
}

func (s *service) NodeNelNet() error {
	servers, err := s.repo.GetServers()
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}

	type probeResult struct {
		name    string
		ip      string
		port    string
		success bool
	}

	maxWorkers := utils.ProbeWorkers()
	sem := make(chan struct{}, maxWorkers)
	ch := make(chan probeResult, len(servers))

	for _, item := range servers {
		sem <- struct{}{}
		go func(name, host, app string, port uint) {
			p := strconv.FormatUint(uint64(port), 10)
			if p == "0" {
				p = "80"
			}
			ok, _ := utils.IpNelNet(host, p)
			ch <- probeResult{name: app + "_" + name, ip: host, port: p, success: ok}
			<-sem
		}(item.Name, item.Host, item.App, item.Port)
	}

	isErrIp := false
	errIpArr := make(map[string]string)
	for range servers {
		r := <-ch
		key := fmt.Sprintf("%s:%s", r.ip, r.port)
		errIpMu.Lock()
		if r.success {
			delete(errIpMap, key)
		} else if _, exists := errIpArr[key]; !exists {
			errIpArr[key] = r.name
			isErrIp = true
		}
		errIpMu.Unlock()
	}
	if isErrIp {
		go YCIpBC(errIpArr, true)
	}
	return nil
}
