package annoy

import (
	"container/heap"
	"fmt"
	"math"
	"os"
	"sort"
	"syscall"
	"unsafe"

	"github.com/edsrzf/mmap-go"
)

type AnnoyIndexInterface interface {
	Unload()
	Load(filename string, prefault bool) error
	GetDistance(i, j int) float32
	GetNnsByItem(item int, n, searchK int) ([]int, []float32)
	GetNnsByVector(v []float32, n, searchK int) ([]int, []float32)
	GetNItems() int
	GetNTrees() int
	Verbose(v bool)
	GetItem(item int) []float32
	SetSeed(seed uint64)
}

type AnnoyIndex[D DistanceMetric] struct {
	distance  D
	f         int
	s         int
	nItems    int32
	nodes     []byte
	nNodes    int32
	nodesSize int32
	roots     []int32
	k         int32
	seed      uint64
	loaded    bool
	verbose   bool
	fd        *os.File
	mmap      mmap.MMap
	onDisk    bool
	built     bool
}

func NewAnnoyIndex[D DistanceMetric](f int, seed uint64) *AnnoyIndex[D] {
	index := &AnnoyIndex[D]{
		f:         f,
		seed:      seed,
		verbose:   false,
		built:     false,
		nodes:     nil,
		nItems:    0,
		nNodes:    0,
		onDisk:    false,
		loaded:    false,
		roots:     []int32{},
		nodesSize: 0,
	}

	index.s = 12 + f*int(unsafe.Sizeof(float32(0)))
	index.k = int32((index.s - 4) / 4)
	index.reinitialize()

	return index
}

func (index *AnnoyIndex[D]) reinitialize() {
	index.fd = nil
	index.nodes = nil
	index.loaded = false
	index.nItems = 0
	index.nNodes = 0
	index.nodesSize = 0 // Reinitialize the index state
	index.onDisk = false
	index.roots = []int32{}
}

func (index *AnnoyIndex[D]) Unload() {
	if index.fd != nil {
		err := index.mmap.Unmap()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmapping memory: %v\n", err)
		}
		err = index.fd.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error closing file descriptor: %v\n", err)
		}
	} else if index.nodes != nil {
		index.nodes = nil
	}

	index.reinitialize()

	if index.verbose {
		fmt.Fprintln(os.Stderr, "unloaded")
	}
}

func (index *AnnoyIndex[D]) Load(filename string, prefault bool) error {
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

	flags := syscall.MAP_SHARED
	if prefault {
		flags |= syscall.MAP_POPULATE
	}

	nodes, err := mmap.Map(f, mmap.RDONLY, 0)
	index.mmap = nodes
	if err != nil {
		return fmt.Errorf("Unable to mmap: %v", err)
	}
	index.nodes = nodes
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
	index.loaded = true
	index.built = true
	index.nItems = m
	if index.verbose {
		fmt.Fprintf(os.Stderr, "found %d roots with degree %d\n", len(index.roots), m)
	}
	return nil
}

func (index *AnnoyIndex[D]) GetDistance(i, j int32) float32 {
	return index.distance.NormalizedDistance(index.getNode(i), index.getNode(j), index.f)
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

func (index *AnnoyIndex[D]) Verbose(v bool) {
	index.verbose = v
}

func (index *AnnoyIndex[D]) GetItem(item int32) []float32 {
	m := index.getNode(item)
	v := make([]float32, index.f)
	copy(v, m.V[:index.f])
	return v
}

func (index *AnnoyIndex[D]) SetSeed(seed uint64) {
	index.seed = seed
}

func (index *AnnoyIndex[D]) getNode(i int32) *Node {
	return GetNodePtr(index.nodes, index.s, i)
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

	sort.Slice(nns, func(i, j int) bool { return nns[i] < nns[j] })
	nnsDist := []Pair{}
	var last int32 = -1
	for _, j := range nns {
		if j == last {
			continue
		}
		last = j
		if index.getNode(j).Descendants == 1 {
			nnsDist = append(nnsDist, Pair{index.distance.Distance(vNode, index.getNode(j), index.f), j})
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
