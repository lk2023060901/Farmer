// Package pathfinding provides A* pathfinding and spatial indexing for the game world.
package pathfinding

import "container/heap"

// Point is a tile coordinate on the map.
type Point struct{ X, Y int }

// FindPath returns the tile path from start to goal using A* (4-directional).
// Returns nil if no path exists.
// isWalkable(x, y) must return false for blocked tiles.
func FindPath(start, goal Point, mapW, mapH int, isWalkable func(x, y int) bool) []Point {
	if start == goal {
		return []Point{start}
	}

	visited := make(map[Point]bool, 64)
	open := &nodeHeap{}
	heap.Push(open, &node{pt: start, g: 0, f: manhattan(start, goal)})

	for open.Len() > 0 {
		cur := heap.Pop(open).(*node)
		if visited[cur.pt] {
			continue
		}
		visited[cur.pt] = true

		if cur.pt == goal {
			return buildPath(cur)
		}

		for _, nb := range adjacent(cur.pt, mapW, mapH) {
			if visited[nb] || !isWalkable(nb.X, nb.Y) {
				continue
			}
			g := cur.g + 1
			heap.Push(open, &node{pt: nb, g: g, f: g + manhattan(nb, goal), parent: cur})
		}
	}
	return nil // no path found
}

// ── helpers ───────────────────────────────────────────────────────────────────

func manhattan(a, b Point) float64 {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	return float64(dx + dy)
}

var dirs = [4]Point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

func adjacent(p Point, w, h int) []Point {
	out := make([]Point, 0, 4)
	for _, d := range dirs {
		nx, ny := p.X+d.X, p.Y+d.Y
		if nx >= 0 && nx < w && ny >= 0 && ny < h {
			out = append(out, Point{nx, ny})
		}
	}
	return out
}

func buildPath(n *node) []Point {
	var path []Point
	for ; n != nil; n = n.parent {
		path = append(path, n.pt)
	}
	// reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// ── priority queue ────────────────────────────────────────────────────────────

type node struct {
	pt     Point
	g, f   float64
	parent *node
	index  int
}

type nodeHeap []*node

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h nodeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}
func (h *nodeHeap) Push(x any) {
	n := x.(*node)
	n.index = len(*h)
	*h = append(*h, n)
}
func (h *nodeHeap) Pop() any {
	old := *h
	n := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	return n
}
