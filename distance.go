package annoy

import (
	"math"
)

type Node struct {
	Descendants int32
	Children    []int32
	Norm        float32
	V           []float32
}

type DistanceMetric interface {
	Distance(x, y *Node, f int) float32
	NormalizeDistance(distance float32) float32
	InitNode(node *Node, f int)
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

func (a Angular) NormalizeDistance(distance float32) float32 {
	return float32(math.Sqrt(math.Max(float64(distance), 0)))
}

func (a Angular) InitNode(node *Node, f int) {
	node.Norm = Dot(node.V[:], node.V[:], f)
}

func (a Angular) Margin(n *Node, y []float32, f int) float32 {
	return Dot(n.V[:], y, f)
}

func (a Angular) PQDistance(distance float32, margin float32, childNr int) float32 {
	if childNr == 0 {
		margin = -margin
	}
	return float32(math.Min(float64(distance), float64(margin)))
}
