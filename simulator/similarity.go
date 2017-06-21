package simulator

import "math"

type similarityFormula func(popularitiesNormalized, popularitiesNormalized) float64

func exponential(fpn1 popularitiesNormalized, fpn2 popularitiesNormalized) float64 {
	var numerator float64
	for k, v := range fpn1 {
		numerator -= v * fpn2[k]
	}
	return 1 - math.Exp(numerator)
}

func cosine(fpn1 popularitiesNormalized, fpn2 popularitiesNormalized) float64 {
	var a, b, ab float64
	for k, v := range fpn1 {
		a += math.Pow(v, 2)
		b += math.Pow(fpn2[k], 2)
		ab += v * fpn2[k]
	}
	return ab / (math.Sqrt(a) * math.Sqrt(b))
}

func (scl smallCellList) calSimilarity(isAccumulated bool, periodID int, filter fileList) [][]float64 {
	s := make([][]float64, len(scl))
	for i := range s {
		s[i] = make([]float64, len(scl))
	}
	for i, sc := range scl {
		for j := i + 1; j < len(scl); j++ {
			if isAccumulated {
				s[i][j] = sc.popularitiesAccumulated[periodID].calSimilarity(scl[j].popularitiesAccumulated[periodID], filter)
			} else {
				s[i][j] = sc.popularitiesPeriod[periodID].calSimilarity(scl[j].popularitiesPeriod[periodID], filter)
			}
			s[j][i] = s[i][j]
		}
	}
	return s
}

func (c *client) calSimilarity(filter fileList) []float64 {
	s := make([]float64, len(cacheStorages))
	for i, cs := range cacheStorages {
		s[i] = c.popularityAccumulated[periodNo].calSimilarity(cs.popularitiesAccumulated[periodNo], filter)
	}
	return s
}

func (p popularities) calSimilarity(fp2 popularities, filter fileList) float64 {
	ifl := p.getFileList()
	ifl = ifl.intersect(filter).intersect(fp2.getFileList())
	if len(ifl) == 0 {
		return 0
	}
	ifp := make(popularities)
	ifp2 := make(popularities)
	for _, f := range ifl {
		ifp[f] = p[f]
		ifp2[f] = fp2[f]
	}
	return formula(ifp.normalize(), ifp2.normalize())
}

func (p popularities) sum() (s int) {
	for _, v := range p {
		s += v
	}
	return
}

func (p popularities) normalize() popularitiesNormalized {
	fpn := make(popularitiesNormalized)
	s := p.sum()
	for k, v := range p {
		fpn[k] = float64(v) / float64(s)
	}
	return fpn
}
