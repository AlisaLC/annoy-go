package annoy

import (
	"container/heap"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/edsrzf/mmap-go"
)

type AnnoyIndexInterface interface {
	Unload() error
	Load(filename string, memory bool) error
	GetDistance(i, j int) float32
	GetNnsByItem(item int, n, searchK int) ([]int, []float32)
	GetNnsByVector(v []float32, n, searchK int) ([]int, []float32)
	GetNItems() int
	GetNTrees() int
	GetItem(item int) []float32
}

type AnnoyIndex[D DistanceMetric] struct {
	distance D
	f        int
	s        int
	nItems   int32
	nodes    []byte
	nNodes   int32
	roots    []int32
	k        int32
	fd       *os.File
	mmap     mmap.MMap
	cache    map[int32]*Node
}

func NewAnnoyIndex[D DistanceMetric](f int) *AnnoyIndex[D] {
	index := &AnnoyIndex[D]{
		f:      f,
		nodes:  nil,
		nItems: 0,
		nNodes: 0,
		roots:  []int32{},
		cache:  make(map[int32]*Node),
	}

	index.s = 12 + f*4
	index.k = int32((index.s - 4) / 4)
	index.reinitialize()

	return index
}

func (index *AnnoyIndex[D]) reinitialize() {
	index.fd = nil
	index.nodes = nil
	index.nItems = 0
	index.nNodes = 0
	index.roots = []int32{}
}

func (index *AnnoyIndex[D]) Unload() error {
	if index.fd != nil {
		if index.mmap != nil {
			err := index.mmap.Unmap()
			if err != nil {
				return fmt.Errorf("Error unmapping memory: %v\n", err)
			}
			index.mmap = nil
		}
		err := index.fd.Close()
		if err != nil {
			return fmt.Errorf("Error closing file descriptor: %v\n", err)
		}
	} else if index.nodes != nil {
		index.nodes = nil
	}

	index.reinitialize()
	return nil
}

func (index *AnnoyIndex[D]) Load(filename string, memory bool) error {
	f, err := os.OpenFile(filename, os.O_RDONLY, 0400)
	if err != nil {
		return fmt.Errorf("Unable to open: %v", err)
	}
	index.fd = f

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("Unable to get size: %v", err)
	}
	size := fi.Size()
	if size == 0 {
		return fmt.Errorf("Size of file is zero")
	}
	if size%int64(index.s) != 0 {
		return fmt.Errorf("Index size is not a multiple of vector size. Ensure you are opening using the same metric you used to create the index.")
	}

	if memory {
		nodes := make([]byte, size)
		_, err = f.Read(nodes)
		index.mmap = nil
		if err != nil {
			return fmt.Errorf("Unable to read: %v", err)
		}
		index.nodes = nodes
	} else {
		nodes, err := mmap.Map(f, mmap.RDONLY, 0)
		index.mmap = nodes
		if err != nil {
			return fmt.Errorf("Unable to mmap: %v", err)
		}
		index.nodes = nodes
	}

	index.nNodes = int32(size / int64(index.s))

	index.roots = []int32{}
	var m int32 = -1
	for i := index.nNodes - 1; i >= 0; i-- {
		k := index.getNode(i).Descendants
		if m == -1 || k == m {
			index.roots = append(index.roots, i)
			m = k
		} else {
			break
		}
	}

	if len(index.roots) > 1 && index.getNode(index.roots[0]).Children[0] == index.getNode(index.roots[len(index.roots)-1]).Children[0] {
		index.roots = index.roots[:len(index.roots)-1]
	}
	index.nItems = m
	return nil
}

func (index *AnnoyIndex[D]) GetDistance(i, j int32) float32 {
	return index.distance.NormalizeDistance(index.distance.Distance(index.getNode(i), index.getNode(j), index.f))
}

func (index *AnnoyIndex[D]) GetNnsByItem(item int32, n, searchK int) ([]int32, []float32) {
	m := index.getNode(item)
	return index.getAllNns(m.V[:], n, searchK)
}

func (index *AnnoyIndex[D]) GetNnsByVector(v []float32, n, searchK int) ([]int32, []float32) {
	return index.getAllNns(v, n, searchK)
}

func (index *AnnoyIndex[D]) GetNItems() int32 {
	return index.nItems
}

func (index *AnnoyIndex[D]) GetNTrees() int {
	return int(len(index.roots))
}

func (index *AnnoyIndex[D]) GetItem(item int32) []float32 {
	m := index.getNode(item)
	v := make([]float32, index.f)
	copy(v, m.V[:index.f])
	return v
}

func (index *AnnoyIndex[D]) getNode(i int32) *Node {
	if index.mmap != nil {
		return GetNodePtr(index.nodes, index.s, i)
	}
	node, ok := index.cache[i]
	if ok {
		return node
	}
	node = GetNodePtr(index.nodes, index.s, i)
	index.cache[i] = node
	if node.V != nil {
		index.distance.InitNode(node, index.f)
	}
	return node
}

func (index *AnnoyIndex[D]) getAllNns(v []float32, n, searchK int) ([]int32, []float32) {
	vNode := &Node{V: make([]float32, index.f)}
	copy(vNode.V, v)
	index.distance.InitNode(vNode, index.f)

	pq := &PriorityQueue{}
	heap.Init(pq)

	if searchK == -1 {
		searchK = n * len(index.roots)
	}

	for _, root := range index.roots {
		heap.Push(pq, &Pair{float32(math.Inf(1)), root})
	}

	nns := []int32{}
	for len(nns) < searchK && pq.Len() > 0 {
		top := heap.Pop(pq).(*Pair)
		d := top.first
		i := top.second
		nd := index.getNode(i)
		if nd.Descendants == 1 && i < index.nItems {
			nns = append(nns, i)
		} else if nd.Descendants <= index.k {
			nns = append(nns, nd.Children[:nd.Descendants]...)
		} else {
			margin := index.distance.Margin(nd, v, index.f)
			heap.Push(pq, &Pair{index.distance.PQDistance(d, margin, 1), nd.Children[1]})
			heap.Push(pq, &Pair{index.distance.PQDistance(d, margin, 0), nd.Children[0]})
		}
	}

	nnSet := make(map[int32]struct{})
	for _, j := range nns {
		nnSet[j] = struct{}{}
	}

	nnsDist := make([]Pair, len(nnSet))
	i := 0
	for j := range nnSet {
		if index.getNode(j).Descendants == 1 {
			nnsDist[i] = Pair{index.distance.Distance(vNode, index.getNode(j), index.f), j}
			i += 1
		}
	}

	m := len(nnsDist)
	p := n
	if p > m {
		p = m
	}

	sort.Slice(nnsDist, func(i, j int) bool { return nnsDist[i].first < nnsDist[j].first })

	result := []int32{}
	distances := []float32{}
	for i := 0; i < p; i++ {
		distances = append(distances, index.distance.NormalizeDistance(nnsDist[i].first))
		result = append(result, nnsDist[i].second)
	}
	return result, distances
}
