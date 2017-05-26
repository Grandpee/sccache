package simulator

import (
	"github.com/cdipaolo/goml/base"
	"github.com/cdipaolo/goml/cluster"
)

const maxIterations = 50

var clusteringModel *cluster.KMeans

func clustering(pl periodList, clusterNum int) (clientList, []int) {
	trainingSet, trainingClientList := pl.getClientFilePopularity()
	clusteringModel = cluster.NewKMeans(clusterNum, maxIterations, trainingSet)
	if clusteringModel.Learn() != nil {
		panic("Clustering error!")
	}
	guesses := clusteringModel.Guesses()

	smallCells = make(smallCellList, len(clusteringModel.Centroids))
	for i := range smallCells {
		smallCells[i] = &smallCell{
			id:                      i,
			clients:                 make(clientMap),
			popularitiesAccumulated: []popularities{make(popularities)},
		}
	}
	for i, c := range trainingClientList {
		c.assignTo(smallCells[guesses[i]])
	}

	return trainingClientList, guesses
}

func (pl periodList) getClientList() clientList {
	ucm := make(clientMap)
	for _, p := range pl {
		for cid, c := range p.clients {
			ucm[cid] = c
		}
	}
	cl := make(clientList, 0, len(ucm))
	for _, c := range ucm {
		cl = append(cl, c)
	}
	return cl
}

func (pl periodList) getClientFilePopularity() ([][]float64, clientList) {
	cl := pl.getClientList()
	data := make([][]float64, len(cl))
	for i, c := range cl {
		data[i] = make([]float64, len(filesList))
		for j, f := range filesList {
			pop := 0
			if popEnd, ok := c.popularityAccumulated[pl[len(pl)-1].id][f]; ok {
				pop = popEnd
				if popStart, ok := c.popularityAccumulated[pl[0].id][f]; ok {
					pop -= popStart
					pop += c.popularityPeriod[pl[0].id][f]
				}
			}
			data[i][j] = float64(pop)
		}
	}
	base.Normalize(data)
	return data, cl
}

func (c *client) getFilePopularity() []float64 {
	data := make([]float64, len(filesList))
	for i, f := range filesList {
		pop := 0
		if popAcc, ok := c.popularityAccumulated[periodNo][f]; ok {
			pop = popAcc
		}
		data[i] = float64(pop)
	}
	base.NormalizePoint(data)
	return data
}

func clusteringWithSimilarity(pl periodList, clusterNum int) (clientList, []int) {
	trainingSet, trainingClientList := pl.getClientsSimilarity()
	clusteringModel = cluster.NewKMeans(clusterNum, maxIterations, trainingSet)
	if clusteringModel.Learn() != nil {
		panic("Clustering error!")
	}
	guesses := clusteringModel.Guesses()

	smallCells = make(smallCellList, len(clusteringModel.Centroids))
	for i := range smallCells {
		smallCells[i] = &smallCell{
			id:                      i,
			clients:                 make(clientMap),
			popularitiesAccumulated: []popularities{make(popularities)},
		}
	}
	for i, c := range trainingClientList {
		c.assignTo(smallCells[guesses[i]])
	}

	return trainingClientList, guesses
}

func (pl periodList) getClientsSimilarity() ([][]float64, clientList) {
	cl := pl.getClientList()
	data := make([][]float64, len(cl))
	cp := make([]popularities, len(cl))
	for i, c := range cl {
		for f, pop := range c.popularityAccumulated[pl[len(pl)-1].id] {
			if popStart, ok := c.popularityAccumulated[pl[0].id][f]; ok {
				pop -= popStart
				pop += c.popularityPeriod[pl[0].id][f]
			}
			cp[i][f] = pop
		}
	}
	for i := 0; i < len(cl); i++ {
		data[i] = make([]float64, len(cl))
		for j := 0; j < len(cl); j++ {
			data[i][j] = cp[i].calSimilarity(cp[j], nil)
			data[j][i] = data[i][j]
		}
	}
	base.Normalize(data)
	return data, cl
}
