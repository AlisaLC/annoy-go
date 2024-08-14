# Fast Read-Only Annoy
## Introduction
This version of Annoy in Go can only be used to load and query Angular indexes created using `Annoy`. I only needed Annoy for loading indexes and querying vectors, So I optimized it to be faster with caching and in-memory storage of nodes instead of `mmap` which `Annoy` does.

You can tell the index to use `mmap` or `memory`. using `memory` with caching it provides lets your queries to be up to twice as fast as native C++ version of `Annoy`.
## Example
```go
import (
	"fmt"
	"testing"
	"time"

	"github.com/AlisaLC/annoy-go"
)

func main() {
    index := annoy.NewAnnoyIndex[annoy.Angular](20)
    index.Load("test.ann", true)
    vector := []float32{
        -19.206135, -0.29148674, -0.19988513, -3.682283,
        11.192975, 8.732752, 3.024612, 3.606107,
        -5.8107805, 7.36837, 3.0414157, -7.0733867,
        -1.7415776, 2.3138094, -14.7966385, -0.75848556,
        8.763108, -1.8256252, 6.524804, -0.6752771,}
    results, distances := index.GetNnsByVector(vector, 1000, -1)
    // first run can be slower since caching is happening.
    // next runs becomes much faster
}
```