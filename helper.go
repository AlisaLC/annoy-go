package annoy

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"syscall"
)

// A Pair is something we manage in a priority queue.
type Pair struct {
	first  float32 // The priority of the node.
	second int32   // The node index.
}

// A PriorityQueue implements heap.Interface and holds Pairs.
type PriorityQueue []*Pair

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want the priority queue to pop the smallest priority, so we use Less here.
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

func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, msg)
}

func RemapMemoryAndTruncate(data *[]byte, fd int, oldSize, newSize int64) bool {
	if err := syscall.Munmap(*data); err != nil {
		return false
	}

	if err := syscall.Ftruncate(fd, newSize); err != nil {
		return false
	}

	newData, err := syscall.Mmap(fd, 0, int(newSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return false
	}

	*data = newData
	return true
}

func GetNodePtr(nodes []byte, size int, i int32) *Node {
	arrSize := (size - 12) / 4
	node := Node{}
	node.Descendants = int32(binary.LittleEndian.Uint32(nodes[size*int(i) : size*int(i)+4]))
	node.Children = make([]int32, arrSize+2)
	for j := 0; j < arrSize+2; j++ {
		node.Children[j] = int32(binary.LittleEndian.Uint32(nodes[size*int(i)+4+4*j : size*int(i)+8+4*j]))
	}
	node.V = make([]float32, arrSize)
	for j := 0; j < arrSize; j++ {
		node.V[j] = math.Float32frombits(binary.LittleEndian.Uint32(nodes[size*int(i)+12+4*j : size*int(i)+16+4*j]))
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

func TwoMeans[Distance DistanceMetric](distance DistanceMetric, nodes []*Node, f int, random *rand.Rand, cosine bool, p, q *Node) {
	iterationSteps := 200
	count := len(nodes)

	i := random.Intn(count)
	j := random.Intn(count - 1)
	if j >= i {
		j++
	}

	distance.CopyNode(p, nodes[i], f)
	distance.CopyNode(q, nodes[j], f)

	if cosine {
		distance.Normalize(p, f)
		distance.Normalize(q, f)
	}
	distance.InitNode(p, f)
	distance.InitNode(q, f)

	ic, jc := 1, 1
	for l := 0; l < iterationSteps; l++ {
		k := random.Intn(count)
		di := float32(ic) * distance.Distance(p, nodes[k], f)
		dj := float32(jc) * distance.Distance(q, nodes[k], f)
		norm := float32(1)
		if cosine {
			norm = distance.GetNorm(nodes[k], f)
		}
		if !(norm > float32(0)) {
			continue
		}
		if di < dj {
			distance.UpdateMean(p, nodes[k], norm, ic, f)
			distance.InitNode(p, f)
			ic++
		} else if dj < di {
			distance.UpdateMean(q, nodes[k], norm, jc, f)
			distance.InitNode(q, f)
			jc++
		}
	}
}
