package annoy

import (
	"encoding/binary"
	"math"
)

type Pair struct {
	first  float32
	second int32
}

type PriorityQueue []*Pair

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].first == pq[j].first {
		return pq[i].second > pq[j].second
	}
	return pq[i].first > pq[j].first
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x any) {
	item := x.(*Pair)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Top() *Pair {
	return (*pq)[0]
}

func GetNodePtr(nodes []byte, size int, i int32) *Node {
	arrSize := (size - 12) / 4
	node := Node{}
	node.Descendants = int32(binary.LittleEndian.Uint32(nodes[size*int(i) : size*int(i)+4]))
	if node.Descendants > 2 && node.Descendants <= int32(arrSize)+2 {
		node.Children = make([]int32, arrSize+2)
		for j := 0; j < arrSize+2; j++ {
			node.Children[j] = int32(binary.LittleEndian.Uint32(nodes[size*int(i)+4+4*j : size*int(i)+8+4*j]))
		}
	} else {
		node.Children = make([]int32, 2)
		for j := 0; j < 2; j++ {
			node.Children[j] = int32(binary.LittleEndian.Uint32(nodes[size*int(i)+4+4*j : size*int(i)+8+4*j]))
		}
		node.V = make([]float32, arrSize)
		for j := 0; j < arrSize; j++ {
			node.V[j] = math.Float32frombits(binary.LittleEndian.Uint32(nodes[size*int(i)+12+4*j : size*int(i)+16+4*j]))
		}
	}
	return &node
}

func Dot(x, y []float32, f int) float32 {
	var s float32
	for z := 0; z < f; z++ {
		s += x[z] * y[z]
	}
	return s
}
