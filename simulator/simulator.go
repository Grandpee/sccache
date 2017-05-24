package simulator

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"
)

var (
	cacheStorages cacheStorageList
	periodNo      int
)

type stats struct {
	downloaded int
	served     int
	dlRate     float64
}

func (s *stats) calRate() {
	s.dlRate = float64(s.downloaded) / float64(s.downloaded+s.served)
}

func (pl periodList) calRate() float64 {
	var dl, sv int
	for _, p := range pl {
		dl += p.downloaded
		sv += p.served
	}
	return float64(dl) / float64(dl+sv)
}

func Simulate(path string) {
	readConfigsFile(path)

	for i, c := range configs {
		readRequestsFile(path, c.PeriodDuration)

		if !c.IsTrained {
			fmt.Println("Clustering...")
			var trainPl periodList = periods[c.TrainStartPeriod : c.TrainEndPeriod+1]
			cl, guesses := trainPl.clustering(c.ClusterNumber)
			writeClusteringResultFiles(path, cl, guesses)
		} else {
			fmt.Println("Read Clustering Model...")
			readClusteringResultFiles(path)
		}

		preProcess(c)
		var pl periodList = periods[c.TestStartPeriod:]
		pl.serve(c)
		pl.postProcess()

		writeResultFile(path, pl, configJSONs[i])
	}
}

func readConfigsFile(path string) {
	f, err := os.Open(path + "configs.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	readConfigs(f)
}

func readRequestsFile(path string, duration time.Duration) {
	f, err := os.Open(path + "requests.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Println("Read Requests...")
	readRequests(f, duration)
}

func readClusteringResultFiles(path string) {
	model := path + "clustering_model.json"
	f, err := os.Open(path + "clustering_result.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	readClusteringResult(model, f)
}

func writeClusteringResultFiles(path string, cl clientList, guesses []int) {
	clusteringModel.PersistToFile(path + "clustering_model.json")
	f, err := os.Create(path + "clustering_result.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for i, c := range cl {
		f.WriteString(c.id + "\t" + strconv.Itoa(guesses[i]) + "\n")
	}
}

func readClustersFile(path string) {
	f, err := os.Open(path + "clusters.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	readClientsAssignment(f)
}

func writeResultFile(path string, pl periodList, cj configJSON) {
	f, err := os.Create(path + cj.SimilarityFormula + "_" + strconv.FormatBool(cj.IsPeriodSimilarity) + "_" + cj.CachePolicy + "_" + strconv.Itoa(cj.FilesLimit) + "_" + strconv.Itoa(cj.FileSize) + "_" + strconv.Itoa(cj.CacheStorageSize) + ".csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	for _, p := range pl {
		f.WriteString(p.end.Format("2006-01-02 15") + "\t" + strconv.FormatFloat(p.dlRate, 'f', 5, 64) + "\n")
	}

	f2, err := os.Create(path + "cluster_file_popularity_" + strconv.Itoa(cj.TrainStartPeriod) + "_" + strconv.Itoa(cj.TrainEndPeriod) + ".csv")
	if err != nil {
		panic(err)
	}
	defer f2.Close()
	for _, sc := range smallCells {
		for i, file := range filesList {
			f2.WriteString(strconv.Itoa(sc.popularitiesAccumulated[pl[len(pl)-1].id][file]))
			if i != len(filesList)-1 {
				f2.WriteString("\t")
			} else {
				f2.WriteString("\n")
			}
		}
	}
}

func preProcess(config config) {
	smallCells.arrangeCooperation(config.CooperationThreshold, config.SimilarityFormula)
	for _, f := range files {
		f.size = config.FileSize
	}
	for _, cs := range cacheStorages {
		cs.size = config.CacheStorageSize
		cs.space = cs.size
	}
}

func (pl periodList) serve(config config) {
	fmt.Println("Start Testing With config:", config)
	for pn, p := range pl {
		p.serve(config, p.popularFiles[:config.FilesLimit])
		if config.IsPeriodSimilarity {
			p.endPeriod(config, pl[pn+1].popularFiles[:config.FilesLimit])
		} else {
			p.endPeriod(config, nil)
		}
	}
	fmt.Println("All Periods Tested")
}

func (p *period) serve(config config, filter fileList) {
	periodNo = p.id
	for _, r := range p.requests {
		t, f, c := r.time, r.file, r.client
		if len(filter) != 0 && !filter.has(f) {
			continue
		}
		if c.smallCell == nil {
			if len(c.popularityAccumulated[periodNo-1]) == 0 {
				cacheStorages.assignNewClient(c, f)
				p.newClients = append(p.newClients, c)
			} else {
				c.assign(config, filter)
			}
		}

		cs := c.smallCell.cacheStorage
		sizeCached, cf := cs.cacheFile(f, config.CachePolicy)
		cf.count++
		cf.lastReq = t
		cs.served += sizeCached
		cs.downloaded += f.size - sizeCached
		p.served += sizeCached
		p.downloaded += f.size - sizeCached
	}
}

func (p *period) endPeriod(config config, filter fileList) {
	p.calRate()
	for _, c := range p.newClients {
		c.assign(config, filter)
	}
	fmt.Println("End Period:", p.end)
}

func (pl periodList) postProcess() {
	for _, p := range pl {
		fmt.Println(p.end, "\t", p.dlRate)
	}
}

func (c *client) assign(config config, filter fileList) {
	if config.IsAssignClustering {
		c.assignWithClusteringModel()
	} else {
		c.assignWithSimilarity(config.SimilarityFormula, filter)
	}
}

func (c *client) assignWithClusteringModel() {
	guess, err := clusteringModel.Predict(c.getFilePopularity())
	if err != nil {
		panic("prediction error")
	}
	c.assignTo(smallCells[int(guess[0])])
}

func (c *client) assignWithSimilarity(fn similarityFormula, filter fileList) {
	sim := c.calSimilarity(fn, filter)
	mi, ms := -1, 0.0
	for i, s := range sim {
		if s > ms {
			mi, ms = i, s
		}
	}
	if mi == -1 {
		c.assignTo(smallCells.leastClients())
	} else {
		c.assignTo(cacheStorages[mi].smallCells.leastClients())
	}
}

func (csl cacheStorageList) assignNewClient(c *client, f *file) {
	scl := csl.smallCellsHasFile(f)
	if len(scl) != 0 {
		c.assignTo(scl.leastClients())
	} else {
		c.assignTo(smallCells.leastClients())
	}
}

func (scl smallCellList) leastClients() *smallCell {
	sort.Slice(scl, func(i, j int) bool { return len(scl[i].clients) < len(scl[j].clients) })
	return scl[0]
}

func (scl smallCellList) arrangeCooperation(threshold float64, fn similarityFormula) cacheStorageList {
	group := make([]smallCellList, 0)
	if threshold < 0 {
		for _, sc := range scl {
			group = append(group, smallCellList{sc})
		}
	} else {
		ok := make([]bool, len(scl))
		sim := scl.calSimilarity(fn, nil)
		for i := 0; i < len(scl); i++ {
			if ok[i] {
				continue
			}
			group = append(group, smallCellList{scl[i]})
			ok[i] = true
			for j := i + 1; j < len(scl); j++ {
				if ok[j] {
					continue
				}
				if sim[i][j] >= threshold {
					group[len(group)-1] = append(group[len(group)-1], scl[j])
					ok[j] = true
				}
			}
		}
	}

	cacheStorages = make(cacheStorageList, len(group))
	for i, g := range group {
		cacheStorages[i] = &cacheStorage{smallCells: make(smallCellList, 0)}
		for _, sc := range g {
			sc.assignTo(cacheStorages[i])
		}
	}

	return cacheStorages
}

func (sc *smallCell) assignTo(cs *cacheStorage) {
	ocs := sc.cacheStorage
	if ocs != nil {
		scl := ocs.smallCells
		for i := range scl {
			if scl[i] == sc {
				scl = append(scl[:i], scl[i+1:]...)
			}
		}
	}
	cs.smallCells = append(cs.smallCells, sc)
	sc.cacheStorage = cs

	for pn, fp := range sc.popularitiesAccumulated {
		if len(cs.popularitiesAccumulated)-1 < pn {
			cs.popularitiesAccumulated = append(cs.popularitiesAccumulated, make(popularities))
		}
		for k, v := range fp {
			if ocs != nil {
				ocs.popularitiesAccumulated[pn][k] -= v
			}
			cs.popularitiesAccumulated[pn][k] += v
		}
	}
}
