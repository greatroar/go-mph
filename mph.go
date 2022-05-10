// Package mph implements a hash/displace minimal perfect hash function.
package mph

import (
	"math/bits"
	"sort"

	"github.com/dgryski/go-metro"
)

// Table stores the values for the hash function
type Table struct {
	Values []int32
	Seeds  []int32
}

type entry struct {
	idx  int32
	hash uint64
}

// New constructs a minimal perfect hash function for the set of keys which returns the index of item in the keys array.
func New(keys []string) *Table {
	size := uint64(len(keys))

	h := make([][]entry, size)
	for idx, k := range keys {
		hash := metro.Hash64Str(k, 0)
		i := reducerange(hash, size)
		// idx+1 so we can identify empty entries in the table with 0
		h[i] = append(h[i], entry{int32(idx) + 1, hash})
	}

	sort.Slice(h, func(i, j int) bool { return len(h[i]) > len(h[j]) })

	values := make([]int32, size)
	seeds := make([]int32, size)

	var hidx int
	for hidx = 0; hidx < len(h) && len(h[hidx]) > 1; hidx++ {
		subkeys := h[hidx]

		var seed uint64
		entries := make(map[uint64]int32)

	newseed:
		for {
			seed++
			for _, k := range subkeys {
				i := reducerange(xorshiftMult64(k.hash+seed), size)
				if entries[i] == 0 && values[i] == 0 {
					// looks free, claim it
					entries[i] = k.idx
					continue
				}

				// found a collision, reset and try a new seed
				for k := range entries {
					delete(entries, k)
				}
				continue newseed
			}

			// made it through; everything got placed
			break
		}

		// mark subkey spaces as claimed ...
		for k, v := range entries {
			values[k] = v
		}

		// ... and assign this seed value for every subkey
		// NOTE(dgryski): While k.hash is different for each entry, reducerange(k.hash, size) is the same.
		// We don't need to loop over the entire slice, we can just take the seed from the first entry.

		i := reducerange(subkeys[0].hash, size)
		seeds[i] = int32(seed)
	}

	// find the unassigned entries in the table
	var free []int
	for i := range values {
		if values[i] == 0 {
			free = append(free, i)
		} else {
			// decrement idx as this is now the final value for the table
			values[i]--
		}
	}

	for hidx < len(h) && len(h[hidx]) > 0 {
		k := h[hidx][0]
		i := reducerange(k.hash, size)
		hidx++

		// take a free slot
		dst := free[0]
		free = free[1:]

		// claim it; -1 because of the +1 at the start
		values[dst] = k.idx - 1

		// store offset in seed as a negative; -1 so even slot 0 is negative
		seeds[i] = -int32(dst + 1)
	}

	return &Table{
		Values: values,
		Seeds:  seeds,
	}
}

// Query looks up an entry in the table and return the index.
func (t *Table) Query(k string) int32 {
	size := uint64(len(t.Values))
	hash := metro.Hash64Str(k, 0)
	i := reducerange(hash, size)
	seed := t.Seeds[i]
	if seed < 0 {
		return t.Values[-seed-1]
	}

	i = reducerange(xorshiftMult64(uint64(seed)+hash), size)
	return t.Values[i]
}

func xorshiftMult64(x uint64) uint64 {
	x ^= x >> 12 // a
	x ^= x << 25 // b
	x ^= x >> 27 // c
	return x * 2685821657736338717
}

// reducerange maps i to an integer in the range [0,n).
// https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
func reducerange(i, n uint64) uint64 {
	hi, _ := bits.Mul64(i, n)
	return hi
}
