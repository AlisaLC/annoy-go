package annoy

import (
	"math"
	"math/rand"
)

type Node struct {
	Descendants int32
	Children    []int32
	Norm        float32
	V           []float32
}

type DistanceMetric interface {
	CopyNode(dest, source *Node, f int)
	Distance(x, y *Node, f int) float32
	NormalizedDistance(x, y *Node, f int) float32
	NormalizeDistance(distance float32) float32
	Normalize(node *Node, f int)
	InitNode(node *Node, f int)
	UpdateMean(mean, newNode *Node, norm float32, c, f int)
	GetNorm(node *Node, f int) float32
	Margin(n *Node, y []float32, f int) float32
	PQDistance(distance, margin float32, childNr int) float32
}

type Angular struct{}

func (a Angular) Distance(x, y *Node, f int) float32 {
	pp := x.Norm
	if pp == 0 {
		pp = Dot(x.V[:], x.V[:], f)
	}
	qq := y.Norm
	if qq == 0 {
		qq = Dot(y.V[:], y.V[:], f)
	}
	ppqq := pp * qq
	if ppqq > 0 {
		pq := Dot(x.V[:], y.V[:], f)
		return 2.0 - 2.0*pq/float32(math.Sqrt(float64(ppqq)))
	}
	return 2.0
}

func (a Angular) NormalizedDistance(x, y *Node, f int) float32 {
	return float32(math.Sqrt(math.Max(float64(a.Distance(x, y, f)), 0)))
}

func (a Angular) NormalizeDistance(distance float32) float32 {
	return float32(math.Sqrt(math.Max(float64(distance), 0)))
}

func (a Angular) CopyNode(dest, source *Node, f int) {
	copy(dest.V[:f], source.V[:f])
}

func (a Angular) GetNorm(node *Node, f int) float32 {
	return float32(math.Sqrt(float64(Dot(node.V[:], node.V[:], f))))
}

func (a Angular) Normalize(node *Node, f int) {
	norm := a.GetNorm(node, f)
	if norm > 0 {
		for z := 0; z < f; z++ {
			node.V[z] /= norm
		}
	}
}

func (a Angular) UpdateMean(mean, newNode *Node, norm float32, c, f int) {
	for z := 0; z < f; z++ {
		mean.V[z] = (mean.V[z]*float32(c) + newNode.V[z]/norm) / float32(c+1)
	}
}

func (a Angular) InitNode(node *Node, f int) {
	node.Norm = Dot(node.V[:], node.V[:], f)
}

func (a Angular) Margin(n *Node, y []float32, f int) float32 {
	return Dot(n.V[:], y, f)
}

func (a Angular) Side(n *Node, y []float32, f int, random *rand.Rand) bool {
	dot := a.Margin(n, y, f)
	if dot != 0 {
		return dot > 0
	}
	return random.Float32() < 0.5
}

func (a Angular) CreateSplit(nodes []*Node, f int, random *rand.Rand, n *Node) {
	p := &Node{}
	q := &Node{}
	TwoMeans[Angular](a, nodes, f, random, true, p, q)
	for z := 0; z < f; z++ {
		n.V[z] = p.V[z] - q.V[z]
	}
	a.Normalize(n, f)
}

func (a Angular) PQDistance(distance float32, margin float32, childNr int) float32 {
	if childNr == 0 {
		margin = -margin
	}
	return float32(math.Min(float64(distance), float64(margin)))
}
